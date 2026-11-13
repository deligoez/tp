package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Tick is how often the server re-derives what it pushes while a page
// listens (spec/1.8.0.md §2).
const Tick = 2 * time.Second

// Payload is one derivation the server pushes: its event name, and the
// payload or the error the derivation failed with.
type Payload struct {
	Event string
	Data  any
	Err   *Error
}

// DeriveConcurrently runs every derivation at once and returns their
// payloads in the order given, so a cold stream waits for the slowest one
// rather than for their sum, and a page still reads them in that order.
func DeriveConcurrently(derivations ...func() Payload) []Payload {
	out := make([]Payload, len(derivations))
	var wg sync.WaitGroup
	for i, derive := range derivations {
		wg.Go(func() { out[i] = derive() })
	}
	wg.Wait()
	return out
}

// subscriberBuffer is how many events a page may fall behind by before the
// server drops it; its EventSource reconnects and gets the current state.
const subscriberBuffer = 32

// sseEvent is one event as it is written to the stream.
type sseEvent struct {
	name string
	data []byte
}

// hub derives the live payloads on a tick while at least one page listens,
// and pushes each to every page only when its content, generated_at left
// out, changed.
type hub struct {
	derive func() []Payload
	tick   time.Duration

	mu      sync.Mutex
	running bool
	order   []string          // event names in the latest derivation's order
	data    map[string][]byte // the last pushed data per event
	keys    map[string][]byte // its content without generated_at
	subs    map[chan sseEvent]struct{}
}

func newHub(derive func() []Payload, tick time.Duration) *hub {
	return &hub{derive: derive, tick: tick, data: map[string][]byte{}, keys: map[string][]byte{}, subs: map[chan sseEvent]struct{}{}}
}

// subscribe registers a page and returns its channel with the current state,
// starting the tick loop when it is not running.
func (h *hub) subscribe() (ch chan sseEvent, current []sseEvent) {
	ch = make(chan sseEvent, subscriberBuffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}
	for _, name := range h.order {
		current = append(current, sseEvent{name: name, data: h.data[name]})
	}
	if !h.running {
		h.running = true
		go h.loop()
	}
	return ch, current
}

// unsubscribe removes a page; a dropped page's channel is already gone.
func (h *hub) unsubscribe(ch chan sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

// loop derives now and then on every tick, and stops once no page listens.
func (h *hub) loop() {
	ticker := time.NewTicker(h.tick)
	defer ticker.Stop()
	for {
		h.publish(h.derive())
		<-ticker.C
		h.mu.Lock()
		if len(h.subs) == 0 {
			h.running = false
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()
	}
}

// publish pushes every payload whose content changed since it was last
// pushed, generated_at left out of the comparison, and keeps the current
// state in this derivation's order.
func (h *hub) publish(payloads []Payload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range payloads {
		data, key, err := encodePayload(p)
		if err != nil {
			data, key, _ = encodePayload(Payload{Event: p.Event, Err: NewError(1, "cannot encode the "+p.Event+" payload: "+err.Error(), "report it: a payload tp derived could not be encoded as JSON")})
		}
		if old, seen := h.keys[p.Event]; seen && bytes.Equal(old, key) {
			continue
		}
		h.data[p.Event], h.keys[p.Event] = data, key
		ev := sseEvent{name: p.Event, data: data}
		for ch := range h.subs {
			select {
			case ch <- ev:
			default:
				delete(h.subs, ch)
				close(ch)
			}
		}
	}
	h.order = derivationOrder(payloads, h.order)
}

// derivationOrder is the events of payloads in their order, followed by the
// events of previous this derivation did not list.
func derivationOrder(payloads []Payload, previous []string) []string {
	order := make([]string, 0, len(payloads))
	listed := map[string]bool{}
	for _, p := range payloads {
		if !listed[p.Event] {
			listed[p.Event] = true
			order = append(order, p.Event)
		}
	}
	for _, name := range previous {
		if !listed[name] {
			order = append(order, name)
		}
	}
	return order
}

// encodePayload is p's data as one line of JSON, and its comparison key: the
// same with a top-level generated_at removed.
func encodePayload(p Payload) (data, key []byte, err error) {
	v := p.Data
	if p.Err != nil {
		v = p.Err
	}
	if data, err = json.Marshal(v); err != nil {
		return nil, nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return data, data, nil
	}
	if _, ok := fields["generated_at"]; !ok {
		return data, data, nil
	}
	delete(fields, "generated_at")
	key, err = json.Marshal(fields)
	return data, key, err
}

// serveEvents streams the hub's events to one page until it disconnects:
// first the current state, then every change.
func (h *hub) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	ch, current := h.subscribe()
	defer h.unsubscribe(ch)
	for _, ev := range current {
		writeEvent(w, ev)
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			writeEvent(w, ev)
			flusher.Flush()
		}
	}
}

// writeEvent writes one Server-Sent Event.
func writeEvent(w http.ResponseWriter, ev sseEvent) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
}

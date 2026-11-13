package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHub_PushesAPayloadOnlyWhenItsContentLessGeneratedAtChanges is §2's push
// rule at the hub, under the race detector: a payload whose generated_at
// alone moves is not pushed again, one whose content moves is, and a page
// subscribing later gets the current state at once.
func TestHub_PushesAPayloadOnlyWhenItsContentLessGeneratedAtChanges(t *testing.T) {
	t.Parallel()
	var tick atomic.Int64
	h := newHub(func() []Payload {
		n := tick.Add(1)
		value := 1
		if n >= 3 {
			value = 2
		}
		return []Payload{{Event: "x", Data: map[string]any{"generated_at": n, "value": value}}}
	}, 10*time.Millisecond)

	ch, current := h.subscribe()
	assert.Empty(t, current)
	var got []string
	deadline := time.After(time.Second)
	for len(got) < 2 {
		select {
		case ev := <-ch:
			got = append(got, string(ev.data))
		case <-deadline:
			t.Fatalf("got %v", got)
		}
	}
	assert.Contains(t, got[0], `"value":1`)
	assert.Contains(t, got[1], `"value":2`)
	select {
	case ev := <-ch:
		t.Fatalf("pushed an unchanged payload: %s", ev.data)
	case <-time.After(50 * time.Millisecond):
	}

	late, current := h.subscribe()
	require.Len(t, current, 1)
	assert.Contains(t, string(current[0].data), `"value":2`)
	h.unsubscribe(late)
	h.unsubscribe(ch)
}

// TestHub_GivesALatePageTheCurrentStateInDerivationOrder is the order a page
// reads its first state in: an event first derived after the others, as the
// graph's export is when a graph is added while the server runs, still
// reaches a page connecting later in the order the derivation lists it, so
// the page sees the graph's export before the task file list.
func TestHub_GivesALatePageTheCurrentStateInDerivationOrder(t *testing.T) {
	t.Parallel()
	var tick atomic.Int64
	h := newHub(func() []Payload {
		if tick.Add(1) == 1 {
			return []Payload{{Event: "b", Data: 1}, {Event: "c", Data: 1}}
		}
		return []Payload{{Event: "a", Data: 1}, {Event: "b", Data: 1}, {Event: "c", Data: 1}}
	}, 10*time.Millisecond)

	ch, _ := h.subscribe()
	defer h.unsubscribe(ch)
	deadline := time.After(time.Second)
	for seen := 0; seen < 3; {
		select {
		case <-ch:
			seen++
		case <-deadline:
			t.Fatalf("saw %d events", seen)
		}
	}

	late, current := h.subscribe()
	defer h.unsubscribe(late)
	names := make([]string, 0, len(current))
	for _, ev := range current {
		names = append(names, ev.name)
	}
	assert.Equal(t, []string{"a", "b", "c"}, names)
}

// TestHub_AClosedStreamReleasesItsSubscription: a page that closes its
// EventSource, as a hidden tab does, leaves the hub at once, so the server
// holds nothing for it and the tick loop can stop.
func TestHub_AClosedStreamReleasesItsSubscription(t *testing.T) {
	t.Parallel()
	h := newHub(func() []Payload { return []Payload{{Event: "x", Data: 1}} }, 10*time.Millisecond)
	srv := httptest.NewServer(http.HandlerFunc(h.serveEvents))
	defer srv.Close()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	subscribers := func() int {
		h.mu.Lock()
		defer h.mu.Unlock()
		return len(h.subs)
	}
	require.Eventually(t, func() bool { return subscribers() == 1 }, time.Second, 5*time.Millisecond)
	cancel()
	_ = resp.Body.Close()
	assert.Eventually(t, func() bool { return subscribers() == 0 }, time.Second, 5*time.Millisecond, "the closed stream left the hub")
}

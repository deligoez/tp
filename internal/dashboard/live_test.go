package dashboard

import (
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

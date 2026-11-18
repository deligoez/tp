package engine

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// acquireWithin takes a slot or gives up after d.
func acquireWithin(dir string, capN int, main bool, d time.Duration) (*GateSlot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return AcquireGateSlot(ctx, dir, capN, main)
}

// TestGateSlots_TheCapIsNeverExceededMainIncluded: six gates, one of them the
// main branch's, never hold more than the two slots at once, and do hold both.
func TestGateSlots_TheCapIsNeverExceededMainIncluded(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "gate-slots")
	var active, peak atomic.Int32
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Go(func() {
			slot, err := acquireWithin(dir, 2, i == 0, 10*time.Second)
			if !assert.NoError(t, err) {
				return
			}
			n := active.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(80 * time.Millisecond)
			active.Add(-1)
			slot.Release()
		})
	}
	wg.Wait()
	assert.Equal(t, int32(2), peak.Load(), "the gates held both slots at once and never a third")
}

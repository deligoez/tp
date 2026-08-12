package engine

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithFileLock_ExcludesConcurrentHolders is the guard for the defect that
// made a whole class of state loss silent: the unlock path used to remove the
// lock file, and flock is held on an INODE, not on a path. Once the file was
// unlinked, a waiter opening the same path created a new inode and locked that
// instead, so two callers ran the critical section at once. On the audit record
// path that cost 4 silently lost rounds in 100 trials of four concurrent
// tp audit --record — four round files written, three of them in the index.
//
// Two details make it discriminating, and both were measured rather than
// guessed. The section is held for 3ms, so contenders are parked in the retry
// backoff when a holder releases — that is the moment the unlink used to hand
// the next waiter a fresh inode. And there are 16 of them, so the window is hit
// every run: this shape fails 9 times out of 10 against the pre-fix lock here
// (10/10 when it was first measured on a quieter machine), where a shorter
// 8-goroutine version failed 1 in 10 and would have shipped as a flake dressed
// up as a guard.
//
// The counters are mutex-guarded because they are shared state and the race
// detector is right about that; the mutex is NOT what serializes the section.
// maxInside is sampled inside the flock-held region, so if the flock stopped
// excluding, two holders would overlap and record 2 regardless of the mutex.
func TestWithFileLock_ExcludesConcurrentHolders(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o600))

	const holders, perHolder = 16, 3
	var mu sync.Mutex
	counter, inside, maxInside := 0, 0, 0

	var wg sync.WaitGroup
	for i := 0; i < holders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perHolder; j++ {
				err := WithFileLock(target, func() error {
					mu.Lock()
					inside++
					if inside > maxInside {
						maxInside = inside
					}
					counter++
					mu.Unlock()

					time.Sleep(3 * time.Millisecond)

					mu.Lock()
					inside--
					mu.Unlock()
					return nil
				})
				assert.NoError(t, err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, holders*perHolder, counter, "every locked section ran exactly once")
	assert.Equal(t, 1, maxInside, "never more than one holder inside the lock at a time")
}

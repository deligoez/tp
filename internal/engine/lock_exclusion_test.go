package engine

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

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
// Honest about what this one proves: it asserts the invariant holds, but it
// does NOT fail against the pre-fix lock — in-process goroutines rarely hit the
// unlink window, and the loss was measured across real processes with the
// binary. TestWithFileLock_LockFileSurvivesRelease below is the discriminating
// guard; this is the invariant it protects, kept because a future change to the
// lock could break exclusion without touching the file lifetime.
func TestWithFileLock_ExcludesConcurrentHolders(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o600))

	const holders, perHolder = 8, 40
	counter := 0
	inside := 0
	maxInside := 0

	var wg sync.WaitGroup
	for i := 0; i < holders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perHolder; j++ {
				err := WithFileLock(target, func() error {
					inside++
					if inside > maxInside {
						maxInside = inside
					}
					counter++
					inside--
					return nil
				})
				assert.NoError(t, err)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, holders*perHolder, counter, "every locked section ran exactly once")
	assert.Equal(t, 1, maxInside, "never more than one holder inside the lock at a time")
}

// TestWithFileLock_LockFileSurvivesRelease pins the mechanism the exclusion
// depends on. Removing the file on release read as tidiness; it is what let a
// waiter lock a different inode at the same path.
func TestWithFileLock_LockFileSurvivesRelease(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o600))

	require.NoError(t, WithFileLock(target, func() error { return nil }))

	lockPath := LockFilePath(target)
	info, err := os.Stat(lockPath)
	require.NoError(t, err, "the lock file must outlive the lock it carried")
	assert.Contains(t, lockPath, filepath.Join(".tp", "locks"),
		"and it lives in the git-ignored lock directory, so keeping it costs nothing")
	assert.Zero(t, info.Size(), "it is a marker, not state")
}

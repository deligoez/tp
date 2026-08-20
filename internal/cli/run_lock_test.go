package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunLock_ChildTPWriteSucceedsWhileRunLockHeld is test 9's first half, run
// across real processes: the run-scoped lock (§3.1.1) is held for the whole
// run, but it is NOT the task-file write lock, so a child unit's own tp write
// lands while the driver holds it. A driver that took the task-file lock for
// the run instead would deadlock every child it spawned.
func TestRunLock_ChildTPWriteSucceedsWhileRunLockHeld(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Run Lock Spec\n## 1. Setup\n### 1.1 Seed\nSeed the project.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "add",
		`{"id":"seed","title":"Seed","estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)

	taskFile := filepath.Join(dir, "spec.tasks.json")

	// The driver holds the run lock for the length of the run.
	acquired := make(chan struct{})
	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		defer close(held)
		lockErr := engine.WithRunLock(taskFile, func() error {
			close(acquired)
			<-release
			return nil
		})
		assert.NoError(t, lockErr, "the driver takes the run lock")
	}()
	<-acquired
	defer func() {
		close(release)
		<-held
	}()

	// A child unit's own tp write, from its own process, mid-run.
	start := time.Now()
	_, stderr, code = runTP(t, dir, "set", "seed", "estimate_minutes=3")
	elapsed := time.Since(start)
	require.Equal(t, 0, code, "a child's tp write succeeds while the run lock is held: %s", stderr)
	assert.Less(t, elapsed, 4*time.Second,
		"the child takes the task-file lock immediately, not after the write-lock timeout")

	stdout, stderr, code := runTP(t, dir, "show", "seed")
	require.Equal(t, 0, code, "show: %s", stderr)
	assert.Contains(t, stdout, `"estimate_minutes": 3`, "the child's write landed during the run")

	// The run lock itself is refused for a second driver over the same cycle.
	var busy *engine.RunLockBusyError
	require.ErrorAs(t, engine.WithRunLock(taskFile, func() error { return nil }), &busy,
		"a second driver over the same task file is refused while the first runs")
	assert.Equal(t, filepath.Join(dir, ".tp", "locks", "run-spec.lock"), busy.LockPath,
		"the refusal names the run-scoped lock")
}

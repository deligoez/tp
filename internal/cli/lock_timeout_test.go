package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSet_LockContentionTimeoutExitsFour holds the task-file write lock from
// this process while a `tp set` subprocess runs. The subprocess must retry with
// backoff past the 1s lock_timeout_seconds, then exit 4 (STATE) with a hint
// naming the centralized lock path (§12.2).
func TestSet_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTask(t, dir, `{"id":"t1","title":"T","depends_on":[],"estimate_minutes":10,"acceptance":"T is done","source_sections":["# Spec"]}`)

	// Resolve the macOS /var -> /private/var symlink so this process and the tp
	// subprocess agree on the absolute task-file path (and thus the lock hash).
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	dir = realDir

	// Shorten the lock timeout to 1s so the test stays fast.
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "lock_timeout_seconds=1")
	require.Equal(t, 0, code, "set lock_timeout_seconds: %s", stderr)

	taskFilePath := filepath.Join(dir, "spec.tasks.json")

	// Hold the write lock from this process for the whole assertion. A lock
	// error (MkdirAll failure or timeout) never calls fn, so it travels over
	// lockErr instead of leaving the wait on acquired to hang.
	acquired := make(chan struct{})
	release := make(chan struct{})
	lockErr := make(chan error, 1)
	go func() {
		lockErr <- engine.WithFileLock(taskFilePath, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	select {
	case <-acquired:
	case err := <-lockErr:
		require.NoError(t, err, "acquire the task-file write lock")
		require.FailNow(t, "WithFileLock returned without calling fn")
	}
	defer close(release)

	_, stderr, code = runTP(t, dir, "set", "t1", "estimate_minutes=5")
	assert.Equal(t, 4, code, "a write held past lock_timeout_seconds exits 4 (STATE): %s", stderr)

	var errObj map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &errObj), "stderr is the tp error object: %s", stderr)
	assert.Contains(t, errObj["error"], "timed out waiting for lock", "error names the contention")
	hint, _ := errObj["hint"].(string)
	assert.Contains(t, hint, filepath.Join(".tp", "locks"), "hint names the lock path")
}

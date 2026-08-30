package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/output"
)

// runLockTarget lays down a task file in a fresh temp project and returns its
// path. The symlinks are resolved because macOS maps /var to /private/var and a
// lock path that disagrees with itself proves nothing.
func runLockTarget(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o600))
	return target
}

// holdRunLock takes the run lock from a goroutine and returns a release func,
// so the caller can observe a SECOND acquirer while the first still holds it.
// A test that only takes the lock once proves nothing.
func holdRunLock(t *testing.T, taskFile string) (release func()) {
	t.Helper()
	acquired := make(chan struct{})
	releaseCh := make(chan struct{})
	held := make(chan struct{})
	go func() {
		defer close(held)
		err := WithRunLock(taskFile, func() error {
			close(acquired)
			<-releaseCh
			return nil
		})
		assert.NoError(t, err, "the first acquirer takes the run lock")
	}()
	<-acquired
	var once bool
	return func() {
		if once {
			return
		}
		once = true
		close(releaseCh)
		<-held
	}
}

// TestRunLockPath_IsRunScopedAndDistinctFromTheTaskFileLock pins §3.1.1's path
// verbatim: .tp/locks/run-<base>.lock, where <base> is the task file with its
// .tasks.json suffix removed. It is a DISTINCT lock from the task-file write
// lock, which is what lets a child write the task file while the driver runs.
func TestRunLockPath_IsRunScopedAndDistinctFromTheTaskFileLock(t *testing.T) {
	target := runLockTarget(t)
	dir := filepath.Dir(target)

	assert.Equal(t, filepath.Join(dir, ".tp", "locks", "run-spec.lock"), RunLockPath(target),
		"the run lock is .tp/locks/run-<base>.lock")
	assert.NotEqual(t, LockFilePath(target), RunLockPath(target),
		"the run lock is a distinct lock from the task-file write lock")
}

// TestWithRunLock_SecondAcquirerIsRefusedWhileHeld is the informative half of
// test 9: a second tp run over the same task file is refused — with the error
// the CLI maps to exit 4 — for as long as the first holds the lock, and the
// lock becomes available again once it releases.
func TestWithRunLock_SecondAcquirerIsRefusedWhileHeld(t *testing.T) {
	target := runLockTarget(t)

	release := holdRunLock(t, target)
	defer release()

	start := time.Now()
	ran := false
	err := WithRunLock(target, func() error {
		ran = true
		return nil
	})
	elapsed := time.Since(start)

	var busy *RunLockBusyError
	require.ErrorAs(t, err, &busy, "a second run over the same task file is refused")
	assert.False(t, ran, "the second run's body never executes")
	assert.Equal(t, RunLockPath(target), busy.LockPath, "the error names the run lock")
	assert.Equal(t, "spec", busy.Base, "the error names the cycle it belongs to")
	assert.Contains(t, busy.Hint(), "spec", "the hint names the cycle")
	// The task-file lock waits out a holder; a run lock must not, because the
	// holder is a whole run and the wait would still end in the same refusal.
	assert.Less(t, elapsed, 2*time.Second, "the refusal is prompt, not a backoff wait")

	release()

	assert.NoError(t, WithRunLock(target, func() error { return nil }),
		"the run lock is available again once the first run releases it")
}

// TestWithRunLock_LeavesTheTaskFileWriteLockFree is the other half of test 9:
// the driver never holds the task-file write lock while a child is in flight,
// so a child's own tp write acquires it normally during the run.
func TestWithRunLock_LeavesTheTaskFileWriteLockFree(t *testing.T) {
	target := runLockTarget(t)

	wrote := false
	err := WithRunLock(target, func() error {
		return WithFileLockTimeout(target, 1, func() error {
			wrote = true
			return nil
		})
	})

	require.NoError(t, err, "a task-file write succeeds while the run lock is held")
	assert.True(t, wrote, "the write's critical section ran")
}

// TestWithRunLock_LockFileSurvivesRelease pins the inode rule the task-file
// lock learned the hard way: flock binds to an inode, so unlinking the marker
// lets the next waiter lock a different inode at the same path and drive the
// same cycle concurrently.
func TestWithRunLock_LockFileSurvivesRelease(t *testing.T) {
	target := runLockTarget(t)

	require.NoError(t, WithRunLock(target, func() error {
		_, err := os.Stat(RunLockPath(target))
		assert.NoError(t, err, "the lock exists under .tp/locks during the run")
		return nil
	}))

	info, err := os.Stat(RunLockPath(target))
	require.NoError(t, err, "the lock file survives release, so a waiter locks the same inode")
	assert.Zero(t, info.Size(), "it is a marker, not state")

	ignore, err := os.ReadFile(filepath.Join(filepath.Dir(target), ".tp", ".gitignore"))
	require.NoError(t, err, "taking the run lock writes .tp/.gitignore")
	assert.Contains(t, string(ignore), "locks/", "the marker never shows up as an untracked change")
}

// TestWithRunLock_FnErrorPropagated: the run's own failure is the caller's, and
// it is not a RunLockBusyError, so the CLI does not report a second driver.
func TestWithRunLock_FnErrorPropagated(t *testing.T) {
	target := runLockTarget(t)

	err := WithRunLock(target, func() error { return assert.AnError })

	assert.ErrorIs(t, err, assert.AnError)
	var busy *RunLockBusyError
	assert.False(t, errors.As(err, &busy), "a failing run is not lock contention")
}

// TestWithRunLock_GitignoreWarningIsANotice pins the channel of the one thing
// WithRunLock reports without failing. output.Notice is the helper that honours
// --quiet; a raw write to os.Stderr does not, so a run started with --quiet
// still printed this advisory.
//
// The failure is induced rather than described: .tp/.gitignore is made a
// directory, so EnsureTPGitignore's ReadFile fails with something that is not
// os.IsNotExist and the warning branch is genuinely taken. Without the loud
// half first, the quiet assertion would hold against a branch that never ran
// and would pass whatever channel the warning used.
func TestWithRunLock_GitignoreWarningIsANotice(t *testing.T) {
	t.Cleanup(func() { output.Configure(false, false, false) })

	blockGitignore := func(target string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Join(filepath.Dir(target), ".tp", ".gitignore"), 0o755))
	}

	loud := runLockTarget(t)
	blockGitignore(loud)
	output.Configure(false, false, true)
	warned := captureAuditRoundNotices(t, func() {
		assert.NoError(t, WithRunLock(loud, func() error { return nil }))
	})
	require.Contains(t, warned, ".gitignore",
		"the induced failure must reach the warning, or the quiet case proves nothing")

	quiet := runLockTarget(t)
	blockGitignore(quiet)
	output.Configure(false, true, true)
	silent := captureAuditRoundNotices(t, func() {
		assert.NoError(t, WithRunLock(quiet, func() error { return nil }))
	})
	assert.Empty(t, silent, "--quiet suppresses the advisory")
}

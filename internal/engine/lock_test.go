package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithFileLock_BasicAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	err := os.WriteFile(lockTarget, []byte("{}"), 0o600)
	require.NoError(t, err)

	executed := false
	err = WithFileLock(lockTarget, func() error {
		executed = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, executed, "callback should have been executed")

	// Lock file should be cleaned up after WithFileLock
	_, err = os.Stat(lockTarget + ".lock")
	assert.True(t, os.IsNotExist(err), "lock file should be removed after WithFileLock")
}

func TestWithFileLock_FnErrorPropagated(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	err := os.WriteFile(lockTarget, []byte("{}"), 0o600)
	require.NoError(t, err)

	err = WithFileLock(lockTarget, func() error {
		return assert.AnError
	})

	assert.ErrorIs(t, err, assert.AnError)
}

func TestWithFileLock_ConcurrentRetriesThenSucceeds(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(lockTarget, []byte("{}"), 0o600))

	acquired := make(chan struct{})
	release := make(chan struct{})

	// First goroutine: hold the lock until signalled.
	go func() {
		_ = WithFileLock(lockTarget, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()

	<-acquired // lock is now held

	// A second writer retries with backoff; it must succeed once the holder
	// releases within the timeout (§12.1), not fail immediately.
	done := make(chan error, 1)
	go func() {
		done <- WithFileLockTimeout(lockTarget, 2, func() error { return nil })
	}()

	// Let the retrier spin a couple of times, then release the holder.
	time.Sleep(100 * time.Millisecond)
	close(release)

	select {
	case err := <-done:
		require.NoError(t, err, "second writer acquires after the holder releases within the timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("second writer did not complete within 5s")
	}
}

func TestWithFileLockTimeout_HeldPastTimeout(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(lockTarget, []byte("{}"), 0o600))

	acquired := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	// A holder that never releases within this test.
	go func() {
		_ = WithFileLock(lockTarget, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()

	<-acquired

	start := time.Now()
	err := WithFileLockTimeout(lockTarget, 1, func() error { return nil })
	elapsed := time.Since(start)

	var timeoutErr *LockTimeoutError
	require.ErrorAs(t, err, &timeoutErr, "a held lock past the timeout returns *LockTimeoutError")
	assert.Contains(t, timeoutErr.LockPath, filepath.Join(".tp", "locks"), "the error names the centralized lock path")
	assert.Greater(t, timeoutErr.Elapsed, time.Duration(0), "the error reports the elapsed wait")
	assert.GreaterOrEqual(t, elapsed, time.Second, "acquisition retried for at least the timeout")
}

func TestWithFileLock_LockLivesUnderTPLocks(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(lockTarget, []byte("{}"), 0o600))

	var held string
	err := WithFileLock(lockTarget, func() error {
		held = LockFilePath(lockTarget)
		_, statErr := os.Stat(held)
		assert.NoError(t, statErr, "lock exists under .tp/locks during the callback")
		return nil
	})
	require.NoError(t, err)

	assert.Contains(t, held, filepath.Join(".tp", "locks"), "lock path is centralized under .tp/locks")
	_, err = os.Stat(held)
	assert.True(t, os.IsNotExist(err), "lock removed after release")
	_, err = os.Stat(lockTarget + ".lock")
	assert.True(t, os.IsNotExist(err), "no sibling lock beside the target")
}

func TestWithFileLock_RemovesStaleSiblingLock(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	require.NoError(t, os.WriteFile(lockTarget, []byte("{}"), 0o600))
	sibling := lockTarget + ".lock"
	require.NoError(t, os.WriteFile(sibling, []byte("stale"), 0o600))

	require.NoError(t, WithFileLock(lockTarget, func() error { return nil }))

	_, err := os.Stat(sibling)
	assert.True(t, os.IsNotExist(err), "stale sibling lock removed on acquisition")
}

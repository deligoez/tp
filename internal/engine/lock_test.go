package engine

import (
	"os"
	"path/filepath"
	"testing"

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

func TestWithFileLock_ConcurrentLockFails(t *testing.T) {
	dir := t.TempDir()
	lockTarget := filepath.Join(dir, "test.tasks.json")
	err := os.WriteFile(lockTarget, []byte("{}"), 0o600)
	require.NoError(t, err)

	acquired := make(chan struct{})
	release := make(chan struct{})

	// First goroutine: hold the lock
	go func() {
		_ = WithFileLock(lockTarget, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()

	<-acquired // wait until lock is held

	// Second attempt while lock is held — should fail immediately (TryLock)
	secondErr := WithFileLock(lockTarget, func() error {
		return nil
	})

	close(release) // release first lock

	require.Error(t, secondErr)
	assert.Contains(t, secondErr.Error(), "locked by another process")
}

package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tp config --extract thins every task file while other tp commands may be
// writing it, so the strip must take that task file's own lock: the
// project-config lock it runs under is keyed to a different path.
func TestStripTaskWorkflowFields_WaitsForTheTaskFileLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tasks.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"workflow":{"quality_gate":"x"},"tasks":[]}`+"\n"), 0o600))

	acquired := make(chan struct{})
	release := make(chan struct{})
	held := make(chan error, 1)
	go func() {
		held <- WithFileLock(path, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	<-acquired

	done := make(chan error, 1)
	go func() { done <- StripTaskWorkflowFields(path, []string{"quality_gate"}) }()

	select {
	case err := <-done:
		close(release)
		t.Fatalf("strip finished while another writer held the task file's lock (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-held)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("strip did not finish after the lock was released")
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "quality_gate")
}

// Query commands read task files lock-free, so the strip must publish its
// rewrite by rename rather than truncating the file in place.
func TestStripTaskWorkflowFields_ReplacesTheFileAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tasks.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"workflow":{"quality_gate":"x"},"tasks":[]}`+"\n"), 0o600))
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, StripTaskWorkflowFields(path, []string{"quality_gate"}))

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, os.SameFile(before, after), "the rewrite replaced the file by rename, not in place")
	assert.Equal(t, os.FileMode(0o600), after.Mode().Perm())
	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

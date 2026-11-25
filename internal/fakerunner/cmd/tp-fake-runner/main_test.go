package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestEditTaskWaitsForTheTaskFileLock pins that the fixture's task-file write
// takes the lock tp's own writers take: a pool driver claims the next unit
// under that lock while a seed child marks its finder wip, and an unlocked
// read-modify-write would drop the claim.
func TestEditTaskWaitsForTheTaskFileLock(t *testing.T) {
	taskFile := filepath.Join(t.TempDir(), "s.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"tasks":[{"id":"a","status":"open"}]}`+"\n"), 0o600))

	held := make(chan struct{})
	release := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- engine.WithFileLock(taskFile, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held

	edited := make(chan error, 1)
	go func() {
		edited <- editTask(taskFile, "a", func(task map[string]any) { task["status"] = "wip" })
	}()
	select {
	case err := <-edited:
		close(release)
		t.Fatalf("editTask returned (%v) while another writer held the task-file lock", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-lockDone)
	require.NoError(t, <-edited)
}

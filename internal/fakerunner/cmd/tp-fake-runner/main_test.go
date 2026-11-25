package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
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

// TestAnswerRequestsRefusesAnUnsetTaskFile pins that a decision unit with no
// TP_FILE fails instead of resolving a project root from the working directory.
func TestAnswerRequestsRefusesAnUnsetTaskFile(t *testing.T) {
	err := answerRequests("", "decision-x")
	require.ErrorContains(t, err, "TP_FILE is unset")
}

// TestAttemptedBeforeReportsAnUnreadableRecord pins that a record the fake
// cannot parse is an error, not "never attempted": reading it as the latter
// re-fires the fail-first, block and found arms on a retry.
func TestAttemptedBeforeReportsAnUnreadableRecord(t *testing.T) {
	dir := t.TempDir()
	file, _, err := fakerunner.Claim(dir)
	require.NoError(t, err)
	_, err = file.WriteString("{not json\n")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	_, err = attemptedBefore(dir, "a")
	require.Error(t, err)
}

// TestWriteSpendLineCreatesAPrivateLog pins the log file's mode at 0o600, the
// mode every other file the fake writes carries.
func TestWriteSpendLineCreatesAPrivateLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs", "1-implement-a.jsonl")
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"tp-fake-runner", "implement", "a", logPath, "0"}

	require.NoError(t, writeSpendLine())
	info, err := os.Stat(logPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

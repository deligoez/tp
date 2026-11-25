package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

func queueFixture(t *testing.T) (root, taskFile string) {
	t.Helper()
	root = t.TempDir()
	git := exec.Command("git", "init", "-q", root)
	require.NoError(t, git.Run())
	require.NoError(t, os.MkdirAll(filepath.Join(root, "spec"), 0o755))
	taskFile = filepath.Join(root, "spec", "feature.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte("{}\n"), 0o600))
	return root, taskFile
}

// A new entry is numbered after the highest Q<n> already in the queue.
func TestFoundQueue_ANewEntryIsNumberedAfterTheHighest(t *testing.T) {
	_, taskFile := queueFixture(t)
	require.NoError(t, WriteFoundQueue(taskFile, []FoundQueueEntry{
		{ID: "Q2", Request: "F1", Location: "a.go:1"},
		{Request: "F2", Location: "b.go:1"},
	}))

	entries, err := ReadFoundQueue(taskFile)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "Q3", entries[1].ID)
}

func TestFoundQueue_AnOpenAndADecidedEntryRoundTripBesideTheTaskFile(t *testing.T) {
	root, taskFile := queueFixture(t)
	open := FoundQueueEntry{
		Request:      "F1",
		Location:     "internal/x.go:12",
		Question:     "Which of the two readings does the spec mean?",
		Alternatives: []string{"the first", "the second"},
		Finders:      []string{"task-a", "task-b"},
	}
	decided := FoundQueueEntry{
		Request:  "F2",
		State:    FoundQueueDecided,
		Location: "spec/feature.md:40",
		Question: "Does the code follow the spec here?",
		Answer:   "conforms: section 3 says so",
	}

	require.NoError(t, UpdateFoundQueue(taskFile, func(q []FoundQueueEntry) ([]FoundQueueEntry, error) {
		return append(q, open, decided), nil
	}))

	path := FoundQueuePath(taskFile)
	assert.Equal(t, filepath.Join(root, "spec", "feature.queue.json"), path, "the queue sits beside the task file")
	got, err := ReadFoundQueue(taskFile)
	require.NoError(t, err)
	open.ID, open.State = "Q1", FoundQueueOpen
	decided.ID = "Q2"
	assert.Equal(t, []FoundQueueEntry{open, decided}, got)

	ignored := exec.Command("git", "-C", root, "check-ignore", "-q", path)
	require.NoError(t, EnsureTPGitignore(filepath.Join(root, ".tp")))
	err = ignored.Run()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "git check-ignore must exit 1: the queue is committed")
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestFoundQueue_AMissingFileIsAnEmptyQueue(t *testing.T) {
	_, taskFile := queueFixture(t)
	got, err := ReadFoundQueue(taskFile)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}

// No queue file counts none; a queue file that exists and cannot be parsed is
// an error for the count, as it is for ReadFoundQueue, not an empty queue.
func TestFoundOpenCount_AnUnparsableQueueIsAnError(t *testing.T) {
	_, taskFile := queueFixture(t)
	open, err := FoundOpenCount(taskFile)
	require.NoError(t, err)
	assert.Equal(t, 0, open)

	require.NoError(t, os.WriteFile(FoundQueuePath(taskFile), []byte("{bad"), 0o600))
	_, err = FoundOpenCount(taskFile)
	require.ErrorContains(t, err, "parse found queue")
	_, err = FoundQueueCounts(taskFile, &model.TaskFile{})
	require.ErrorContains(t, err, "parse found queue")
}

func TestFoundQueue_AWriteWaitsForTheTaskFileLock(t *testing.T) {
	_, taskFile := queueFixture(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(LockFilePath(taskFile)), 0o755))
	held := flock.New(LockFilePath(taskFile))
	locked, err := held.TryLock()
	require.NoError(t, err)
	require.True(t, locked)

	done := make(chan error, 1)
	go func() {
		done <- UpdateFoundQueue(taskFile, func(q []FoundQueueEntry) ([]FoundQueueEntry, error) {
			return append(q, FoundQueueEntry{Location: "a.go:1"}), nil
		})
	}()

	select {
	case err := <-done:
		t.Fatalf("the queue was written while another process held the task-file lock (err %v)", err)
	case <-time.After(200 * time.Millisecond):
	}
	_, statErr := os.Stat(FoundQueuePath(taskFile))
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	require.NoError(t, held.Unlock())
	require.NoError(t, <-done)
	got, err := ReadFoundQueue(taskFile)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

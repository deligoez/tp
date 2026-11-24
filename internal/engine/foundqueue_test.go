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


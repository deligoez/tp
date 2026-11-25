package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestSettleFoundQueue_AFailedQueueWriteLeavesTheTaskFileAsItWas: settling an
// entry writes the made task into the task file, then the queue. When the queue
// write failed, the task file kept the task while the entry stayed open, so a
// retry made a second task (found-q1-2). Both writes stand or neither does.
func TestSettleFoundQueue_AFailedQueueWriteLeavesTheTaskFileAsItWas(t *testing.T) {
	taskFile := seedOneOpenFoundEntry(t)
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	orig := writeFoundQueue
	writeFoundQueue = func(string, []engine.FoundQueueEntry) error { return errors.New("disk full") }
	t.Cleanup(func() { writeFoundQueue = orig })

	_, err = settleFoundQueue(taskFile, []engine.FoundSettleRow{{Entry: "Q1"}})
	require.Error(t, err)

	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the task file is restored when the queue write fails")
}

// TestSettleFoundQueue_TheRestoreIsAnAtomicReplace: query commands read the
// task file lock-free, so the restore must swap in a whole file the way
// model.WriteTaskFile does, never truncate and rewrite the one a reader has
// open. A replace leaves a different file at the path than the one present
// when the queue write failed; an in-place write leaves the same one.
func TestSettleFoundQueue_TheRestoreIsAnAtomicReplace(t *testing.T) {
	taskFile := seedOneOpenFoundEntry(t)

	var written os.FileInfo
	orig := writeFoundQueue
	writeFoundQueue = func(string, []engine.FoundQueueEntry) error {
		info, err := os.Stat(taskFile)
		require.NoError(t, err)
		written = info
		return errors.New("disk full")
	}
	t.Cleanup(func() { writeFoundQueue = orig })

	_, err := settleFoundQueue(taskFile, []engine.FoundSettleRow{{Entry: "Q1"}})
	require.Error(t, err)

	restored, err := os.Stat(taskFile)
	require.NoError(t, err)
	require.NotNil(t, written)
	assert.False(t, os.SameFile(written, restored), "the restore replaces the task file instead of rewriting it in place")
	assert.Equal(t, os.FileMode(0o600), restored.Mode().Perm())
	leftovers, err := filepath.Glob(taskFile + ".*.tmp")
	require.NoError(t, err)
	assert.Empty(t, leftovers, "the restore leaves no temp file behind")
}

// seedOneOpenFoundEntry writes a task file with one task and a found queue
// holding one open entry, Q1, and returns the task file's path.
func seedOneOpenFoundEntry(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"version":1,"spec":"spec.md","tasks":[
		{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`), 0o600))
	require.NoError(t, engine.WriteFoundQueue(taskFile, []engine.FoundQueueEntry{
		{State: engine.FoundQueueOpen, Request: "F1", Location: "x.go:4", Question: "open a task?",
			Reproduction: "go test ./x", Expected: "exit 0", Observed: "exit 1", Finders: []string{"t1"}},
	}))
	return taskFile
}

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
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"version":1,"spec":"spec.md","tasks":[
		{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`), 0o600))
	require.NoError(t, engine.WriteFoundQueue(taskFile, []engine.FoundQueueEntry{
		{State: engine.FoundQueueOpen, Request: "F1", Location: "x.go:4", Question: "open a task?",
			Reproduction: "go test ./x", Expected: "exit 0", Observed: "exit 1", Finders: []string{"t1"}},
	}))
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

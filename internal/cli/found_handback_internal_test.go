package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// A handback whose queue write fails leaves nothing of the handback on disk:
// the task file written first is put back, so the task is not left done and
// handed back with no entry holding its finders, and a retry is not refused
// as already done. Not parallel: it swaps writeHandbackQueue.
func TestFoundHandback_AFailedQueueWriteLeavesTheTaskOpen(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"version":1,"spec":"spec.md","tasks":[
		{"id":"found-f1","title":"F1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"],"origin":{"location":"a.go:1"}},
		{"id":"t1","title":"T1","status":"open","depends_on":["found-f1"],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`), 0o600))
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	writeHandbackQueue = func(string, []engine.FoundQueueEntry) error { return errors.New("queue write failed") }
	t.Cleanup(func() { writeHandbackQueue = engine.WriteFoundQueue })

	_, err = handBackFoundTask(dir, taskFile, "found-f1", "TestA passes at HEAD")
	require.ErrorContains(t, err, "queue write failed")
	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the task file written before the queue is put back")
	_, err = os.Stat(engine.FoundQueuePath(taskFile))
	assert.ErrorIs(t, err, os.ErrNotExist, "no queue file is left")

	writeHandbackQueue = engine.WriteFoundQueue
	entry, err := handBackFoundTask(dir, taskFile, "found-f1", "TestA passes at HEAD")
	require.NoError(t, err, "the retry is not refused as already done")
	assert.Equal(t, []string{"t1"}, entry.Finders)
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "found-f1")
	require.NoError(t, err)
	assert.True(t, task.HandedBack)
}

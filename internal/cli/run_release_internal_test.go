package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestReleaseBatchFunc_PutsClaimedTasksBackToOpen: a batch that failed to
// start goes back exactly as it was before the claim — open, no start time,
// no duration source, solo untouched — and a task that is not wip is left
// alone (spec/backlog/worktree-setup.md §1).
func TestReleaseBatchFunc_PutsClaimedTasksBackToOpen(t *testing.T) {
	t.Parallel()
	taskFile := filepath.Join(t.TempDir(), "s.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"spec":"s.md","tasks":[
 {"id":"a","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x","source_sections":["x"]},
 {"id":"b","title":"B","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x","source_sections":["x"],"solo":true},
 {"id":"c","title":"C","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"x","source_sections":["x"],"closed_reason":"done"}]}`), 0o600))

	require.NoError(t, claimTasksFunc(taskFile)([]string{"a", "b"}))
	require.NoError(t, releaseTasksFunc(taskFile)([]string{"a", "b", "c"}))

	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	for _, id := range []string{"a", "b"} {
		task, _, findErr := model.FindTask(tf, id)
		require.NoError(t, findErr)
		assert.Equal(t, model.StatusOpen, task.Status, "%s is released", id)
		assert.Nil(t, task.StartedAt, "%s carries no claim time", id)
		assert.Empty(t, task.DurationSource, "%s carries no duration source", id)
	}
	a, _, _ := model.FindTask(tf, "a")
	b, _, _ := model.FindTask(tf, "b")
	c, _, _ := model.FindTask(tf, "c")
	assert.False(t, a.Solo, "a release never marks a task solo")
	assert.True(t, b.Solo, "a task that was solo stays solo")
	assert.Equal(t, model.StatusDone, c.Status, "a task that is not wip is left alone")
}

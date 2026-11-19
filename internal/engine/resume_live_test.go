package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// holdLiveRunLock holds the cycle's run lock until the test ends, as a live run
// does.
func holdLiveRunLock(t *testing.T, taskFile string) {
	t.Helper()
	acquired, release, held := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(held)
		assert.NoError(t, WithRunLock(taskFile, func() error {
			close(acquired)
			<-release
			return nil
		}))
	}()
	<-acquired
	t.Cleanup(func() {
		close(release)
		<-held
	})
}

// TestTaskExport_ALivePoolReachesTheExportAndTheList: tp export and the
// dashboard's task-file list read the cycle as tp resume does, so beside a
// live pool they name its members and offer nothing to start.
func TestTaskExport_ALivePoolReachesTheExportAndTheList(t *testing.T) {
	root, _, taskFile, _ := batchProject(t, rollingTasks(2, "alpha", "beta"))
	require.NoError(t, editTasks(taskFile, []string{"alpha"}, func(task *model.Task) { task.Status = model.StatusWIP }))
	rec, err := NewRunRecorder(root, taskFile, "live", PhaseImplement)
	require.NoError(t, err)
	require.NoError(t, rec.JoinPool("alpha"))
	holdLiveRunLock(t, taskFile)

	exp, err := TaskExportOf(root, taskFile)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, exp.Resume.RunningUnits)
	assert.Empty(t, exp.Resume.NextUnits, "the pool starts what comes next")
	assert.Nil(t, exp.Resume.NextAction.Command, "no command that takes a task")

	row, _ := taskFileRow(root, "tasks.json", taskFile, resumeReads{})
	assert.Contains(t, row.NextStep, "in progress")
}

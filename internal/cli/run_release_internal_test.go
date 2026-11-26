package cli

import (
	"os"
	"path/filepath"
	"reflect"
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

// heldRunTaskFile is a task file whose task a is held: open with waiting set.
func heldRunTaskFile(t *testing.T) string {
	t.Helper()
	taskFile := filepath.Join(t.TempDir(), "s.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"spec":"s.md","tasks":[
 {"id":"a","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x","source_sections":["x"],"waiting":"the release tag"}]}`), 0o600))
	return taskFile
}

func waitingOf(t *testing.T, taskFile string) string {
	t.Helper()
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "a")
	require.NoError(t, err)
	return task.Waiting
}

// TestRunClaimClearsAHold: a run that claims a held task ends the hold, so the
// wip task is not read as parked (spec/backlog/filled-pool.md, Decision 3).
func TestRunClaimClearsAHold(t *testing.T) {
	t.Parallel()
	taskFile := heldRunTaskFile(t)
	require.NoError(t, claimTasksFunc(taskFile)([]string{"a"}))
	assert.Empty(t, waitingOf(t, taskFile))
}

// TestRunReleaseClearsWaiting: a released task goes back to open without the
// parking text, which on an open task would hold it.
func TestRunReleaseClearsWaiting(t *testing.T) {
	t.Parallel()
	taskFile := heldRunTaskFile(t)
	require.NoError(t, claimTasksFunc(taskFile)([]string{"a"}))
	setWaiting(t, taskFile, "a long run")
	require.NoError(t, releaseTasksFunc(taskFile)([]string{"a"}))
	assert.Empty(t, waitingOf(t, taskFile))
}

// TestRunSoloReopenClearsWaiting: the solo reopen is a path back to open too.
func TestRunSoloReopenClearsWaiting(t *testing.T) {
	t.Parallel()
	taskFile := heldRunTaskFile(t)
	require.NoError(t, claimTasksFunc(taskFile)([]string{"a"}))
	setWaiting(t, taskFile, "a long run")
	require.NoError(t, reopenSoloFunc(taskFile)("a", ""))
	assert.Empty(t, waitingOf(t, taskFile))
}

// TestRunSoloReopenRecordsTheConflictBranch: a solo reopen over a conflict
// records the unit branch on the task, where it survives a driver restart
// for the unit that resolves it (spec/backlog/worktree-dispatch.md §2.2).
func TestRunSoloReopenRecordsTheConflictBranch(t *testing.T) {
	t.Parallel()
	taskFile := heldRunTaskFile(t)
	require.NoError(t, claimTasksFunc(taskFile)([]string{"a"}))
	require.NoError(t, reopenSoloFunc(taskFile)("a", "tp-unit/c/r/a"))
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "a")
	require.NoError(t, err)
	assert.Equal(t, "tp-unit/c/r/a", task.ConflictBranch)
	assert.True(t, task.Solo)
	assert.Equal(t, model.StatusOpen, task.Status)
}

// TestRunSoloReopenKeepsARecordedConflictBranch: a later reopen that names no
// branch (a failed gate, tp land) keeps the one recorded, since that branch
// may still hold the only copy of the work.
func TestRunSoloReopenKeepsARecordedConflictBranch(t *testing.T) {
	t.Parallel()
	taskFile := heldRunTaskFile(t)
	require.NoError(t, reopenSoloFunc(taskFile)("a", "tp-unit/c/r/a"))
	require.NoError(t, claimTasksFunc(taskFile)([]string{"a"}))
	require.NoError(t, reopenSoloFunc(taskFile)("a", ""))
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "a")
	require.NoError(t, err)
	assert.Equal(t, "tp-unit/c/r/a", task.ConflictBranch)
}

// TestRunRenameConflictBranchFollowsTheSetAside: once tp run sets a unit
// branch aside, the task recording its old name as conflict_branch names the
// new one, its conflict_sha unchanged; a name no task records changes nothing.
func TestRunRenameConflictBranchFollowsTheSetAside(t *testing.T) {
	t.Parallel()
	taskFile := heldRunTaskFile(t)
	require.NoError(t, reopenSoloFunc(taskFile)("a", "tp-unit/c/r/a"))
	read := func() *model.Task {
		tf, err := model.ReadTaskFile(taskFile)
		require.NoError(t, err)
		task, _, err := model.FindTask(tf, "a")
		require.NoError(t, err)
		return task
	}
	// The scratch task file has no git, so the recorded tip is set by hand:
	// a rename must carry it over untouched.
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	tf.Tasks[0].ConflictSHA = "0123abcd"
	require.NoError(t, model.WriteTaskFile(taskFile, tf))
	sha := read().ConflictSHA
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	require.NoError(t, renameConflictBranchFunc(taskFile)("tp-unit/c/other/a", "tp-unit/c/other-1/a"))
	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "a name no task records leaves the file unwritten")
	require.NoError(t, renameConflictBranchFunc(taskFile)("tp-unit/c/r/a", "tp-unit/c/r-1/a"))
	assert.Equal(t, "tp-unit/c/r-1/a", read().ConflictBranch)
	assert.Equal(t, sha, read().ConflictSHA, "a rename leaves the recorded tip")
}

// TestRunDriverOptionsSupplyEveryCallback: the engine reaches the task file
// only through DriverOptions' callbacks, and a nil one is skipped silently,
// so tp run supplies every one of them.
func TestRunDriverOptionsSupplyEveryCallback(t *testing.T) {
	t.Parallel()
	opts := runDriverOptions(heldRunTaskFile(t), "spec.md")
	v := reflect.ValueOf(opts)
	for i := range v.NumField() {
		// Unexported func fields are the engine's own test seams.
		if f := v.Field(i); f.Kind() == reflect.Func && v.Type().Field(i).IsExported() {
			assert.False(t, f.IsNil(), "DriverOptions.%s is not supplied", v.Type().Field(i).Name)
		}
	}
}

func setWaiting(t *testing.T, taskFile, waiting string) {
	t.Helper()
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "a")
	require.NoError(t, err)
	task.Waiting = waiting
	require.NoError(t, model.WriteTaskFile(taskFile, tf))
}

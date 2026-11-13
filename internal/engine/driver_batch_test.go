package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// twoBatchableTasks are two ready tasks with disjoint touches under
// run_max_parallel 2, so the oracle returns them as one batch.
const twoBatchableTasks = `{"spec":"s.md","workflow":{"run_max_parallel":2},"tasks":[
 {"id":"alpha","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"],"touches":["alpha.txt"]},
 {"id":"beta","title":"B","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"],"touches":["beta.txt"]}]}`

// batchProject is seamProject over a real git repository with everything
// committed, which a batch's worktrees need.
func batchProject(t *testing.T, tasksJSON string) (root, spec, taskFile, records string) {
	t.Helper()
	root, spec, taskFile, records = seamProject(t, tasksJSON)
	require.NoError(t, os.Remove(filepath.Join(root, ".git")))
	recordRounds(t, spec, 0, 2, true)
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "t@example.com")
	gitIn(t, root, "config", "user.name", "t")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "base")
	return root, spec, taskFile, records
}

// fakeCloser closes a task the way tp done does after its gate, failing the
// gate for the ids in fail; it records the commits each close was given.
type fakeCloser struct {
	mu     sync.Mutex
	fail   map[string]bool
	closed map[string][]string
}

func (c *fakeCloser) close(taskFile string) CloseFunc {
	return func(id, _ string, commits []string, _ string) error {
		if c.fail[id] {
			return errors.New("quality gate failed")
		}
		c.mu.Lock()
		c.closed[id] = commits
		c.mu.Unlock()
		return WithFileLock(taskFile, func() error {
			tf, err := model.ReadTaskFile(taskFile)
			if err != nil {
				return err
			}
			task, _, err := model.FindTask(tf, id)
			if err != nil {
				return err
			}
			task.Status = model.StatusDone
			task.CommitSHAs = commits
			return model.WriteTaskFile(taskFile, tf)
		})
	}
}

// editTasks applies edit to each named task under the task file's lock, the
// test's stand-in for the CLI's claim and reopen.
func editTasks(taskFile string, ids []string, edit func(*model.Task)) error {
	return WithFileLock(taskFile, func() error {
		tf, err := model.ReadTaskFile(taskFile)
		if err != nil {
			return err
		}
		for _, id := range ids {
			task, _, findErr := model.FindTask(tf, id)
			if findErr != nil {
				return findErr
			}
			edit(task)
		}
		return model.WriteTaskFile(taskFile, tf)
	})
}

func driveBatch(t *testing.T, root, spec, taskFile string, closer *fakeCloser) DriverResult {
	t.Helper()
	done := make(chan DriverResult, 1)
	go func() {
		res, err := RunDriver(&DriverOptions{
			Root: root, TaskFile: taskFile, Spec: spec, Workflow: *driverWorkflow(), CloseTask: closer.close(taskFile),
			ClaimTasks: func(ids []string) error {
				return editTasks(taskFile, ids, func(task *model.Task) {
					if task.Status == model.StatusOpen {
						task.Status = model.StatusWIP
					}
				})
			},
			ReopenSolo: func(id string) error {
				return editTasks(taskFile, []string{id}, func(task *model.Task) {
					task.Status, task.Solo = model.StatusOpen, true
				})
			},
		})
		assert.NoError(t, err)
		done <- res
	}()
	select {
	case res := <-done:
		return res
	case <-time.After(90 * time.Second):
		t.Fatal("the driver loop did not terminate")
		return DriverResult{}
	}
}

func taskByID(t *testing.T, taskFile, id string) *model.Task {
	t.Helper()
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, id)
	require.NoError(t, err)
	return task
}

// TestRunDriver_ImplementBatchRunsSideBySideAndIntegrates: the two units run
// at once, each in its own worktree; the driver picks both onto the main
// branch, closes each with the picked sha, and removes the worktrees
// (spec/backlog/parallel-implement.md §2.3, §2.4).
func TestRunDriver_ImplementBatchRunsSideBySideAndIntegrates(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, twoBatchableTasks)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "300")
	closer := &fakeCloser{closed: map[string][]string{}}

	res := driveBatch(t, root, spec, taskFile, closer)
	assert.Equal(t, StopConverged, res.StopReason)

	recs := invocations(t, records)
	require.Len(t, recs, 2)
	assert.True(t, overlapped(&recs[0], &recs[1]), "a batch's units run side by side")

	log := gitIn(t, root, "log", "--format=%s", "main")
	assert.Contains(t, log, "unit alpha")
	assert.Contains(t, log, "unit beta")
	assert.FileExists(t, filepath.Join(root, "alpha.txt"))
	assert.FileExists(t, filepath.Join(root, "beta.txt"))
	for _, id := range []string{"alpha", "beta"} {
		require.Len(t, closer.closed[id], 1)
		assert.Equal(t, closer.closed[id][0], gitIn(t, root, "log", "-1", "--format=%H", "--grep", "unit "+id, "main"),
			"the close names the sha picked onto the main branch")
	}
	assert.NotContains(t, gitIn(t, root, "worktree", "list"), "tp-worktrees", "the worktrees are removed")
	assert.Empty(t, gitIn(t, root, "branch", "--list", unitBranchPrefix+"*"))
	pending, err := ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// TestRunDriver_APendingCloseForADoneTaskIsDropped: a driver that died after
// closing a task but before removing its pending close leaves both; the next
// run drops the request rather than picking again and reopening a done task.
func TestRunDriver_APendingCloseForADoneTaskIsDropped(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, oneDoneTask)
	require.NoError(t, WritePendingClose(root, taskFile, &PendingClose{ID: "t", Commits: []string{gitIn(t, root, "rev-parse", "HEAD")}}))
	closer := &fakeCloser{closed: map[string][]string{}}

	res := driveBatch(t, root, spec, taskFile, closer)
	assert.Equal(t, StopConverged, res.StopReason)
	task := taskByID(t, taskFile, "t")
	assert.Equal(t, model.StatusDone, task.Status)
	assert.False(t, task.Solo)
	pending, err := ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// TestAssembleResume_NamesPendingCloses: a pending close a crashed run left is
// named by tp resume until a run integrates it (§2.6).
func TestAssembleResume_NamesPendingCloses(t *testing.T) {
	dir, spec, tfp := setupResumeProject(t, twoBatchableTasks)
	assert.Empty(t, assemble(t, dir, spec, tfp).PendingCloses)
	require.NoError(t, WritePendingClose(CanonicalProjectRoot(dir), tfp, &PendingClose{ID: "beta", Commits: []string{"abc"}}))
	assert.Equal(t, []string{"beta"}, assemble(t, dir, spec, tfp).PendingCloses)
}

// TestUnitAttemptPrompt_ABatchedUnitIsToldItsTask: a child spawned in a
// worktree is told to run the brief that names its task, not tp next.
func TestUnitAttemptPrompt_ABatchedUnitIsToldItsTask(t *testing.T) {
	t.Parallel()
	solo := &unitAttempt{unit: NextUnit{Kind: UnitImplement, ID: "alpha"}}
	assert.Contains(t, solo.prompt(), "`tp next --brief`")
	batched := &unitAttempt{unit: NextUnit{Kind: UnitImplement, ID: "alpha"}, dir: "/tmp/wt"}
	assert.Contains(t, batched.prompt(), "`tp brief alpha`")
}

// TestRunDriver_ImplementBatchConflictReopensSolo: two units that write the
// same file conflict when the second is picked; the main branch keeps the
// first, and the second reopens marked solo and then runs alone.
func TestRunDriver_ImplementBatchConflictReopensSolo(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, twoBatchableTasks)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvFile, "shared.txt")
	closer := &fakeCloser{closed: map[string][]string{}}

	res := driveBatch(t, root, spec, taskFile, closer)
	assert.Equal(t, StopConverged, res.StopReason)

	beta := taskByID(t, taskFile, "beta")
	assert.True(t, beta.Solo, "the task that failed to integrate is marked solo")
	assert.Equal(t, model.StatusDone, beta.Status, "and its solo rerun closed it")
	recs := invocations(t, records)
	require.Len(t, recs, 3, "two side by side, then beta alone")
	assert.Equal(t, "beta", recs[2].Env[EnvUnitID])
	data, err := os.ReadFile(filepath.Join(root, "shared.txt"))
	require.NoError(t, err)
	assert.Equal(t, "alpha\n", string(data), "the main branch keeps the first pick only")
	assert.Empty(t, strings.TrimSpace(gitIn(t, root, "status", "--porcelain", "--untracked-files=no", "--", "shared.txt")))
}

// TestRunDriver_ImplementBatchGateFailureReopensSolo: a pick whose gate fails
// is taken back off the main branch and the task reopens solo, while the
// task file keeps the other unit's close.
func TestRunDriver_ImplementBatchGateFailureReopensSolo(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, twoBatchableTasks)
	t.Setenv(fakerunner.EnvDurable, "1,1,0")
	t.Setenv(fakerunner.EnvExits, "0,0,1,1")
	closer := &fakeCloser{closed: map[string][]string{}, fail: map[string]bool{"alpha": true}}

	res := driveBatch(t, root, spec, taskFile, closer)
	assert.Equal(t, StopUnitFailure, res.StopReason, "alpha's solo rerun writes nothing and fails")

	alpha := taskByID(t, taskFile, "alpha")
	assert.True(t, alpha.Solo)
	assert.NoFileExists(t, filepath.Join(root, "alpha.txt"), "the failed pick is taken back")
	assert.FileExists(t, filepath.Join(root, "beta.txt"))
	assert.Equal(t, model.StatusDone, taskByID(t, taskFile, "beta").Status)
}

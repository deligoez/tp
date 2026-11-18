package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// A setup that outlives its timeout is cut off and reported as timed out,
// with no exit code. The driver passes the gate timeout, whose 30 s floor is
// too long for a test, so this calls the runner with a short one directly.
func TestRunSetupCommand_TimesOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	start := time.Now()
	code, cause := runSetupCommand(context.Background(), "sleep 5", dir, os.Environ(), 200*time.Millisecond, filepath.Join(dir, "setup.log"))
	assert.Nil(t, code, "a timed-out setup has no exit code")
	assert.Contains(t, cause, "timed out")
	assert.Less(t, time.Since(start), 3*time.Second, "the wait ends at the timeout")
}

// setupProject is batchProject with worktree_setup_cmd committed to the
// project layer, and cache/ ignored so a setup may write there.
func setupBatchProject(t *testing.T, command string) (root, spec, taskFile, records string) {
	t.Helper()
	root, spec, taskFile, records = batchProject(t, twoBatchableTasks)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("cache/\n.tp/runs/\n.tp/run-*.json\n"), 0o600))
	if command != "" {
		cfg, err := json.Marshal(map[string]any{"workflow": map[string]any{"worktree_setup_cmd": command}})
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp"), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"), cfg, 0o600))
	}
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "setup config")
	return root, spec, taskFile, records
}

// driveSetupBatch runs the loop with a release function and returns whatever
// it ended with, error included. Each mod adjusts the options first.
func driveSetupBatch(t *testing.T, root, spec, taskFile string, mods ...func(*DriverOptions)) (DriverResult, error) {
	t.Helper()
	closer := &fakeCloser{closed: map[string][]string{}}
	type outcome struct {
		res DriverResult
		err error
	}
	done := make(chan outcome, 1)
	opts := &DriverOptions{
		Root: root, TaskFile: taskFile, Spec: spec, Workflow: *driverWorkflow(), CloseTask: closer.close(taskFile),
		ClaimTasks: func(ids []string) error {
			return editTasks(taskFile, ids, func(task *model.Task) {
				if task.Status == model.StatusOpen {
					now := time.Now()
					task.Status, task.StartedAt = model.StatusWIP, &now
				}
			})
		},
		ReopenSolo: func(id string) error {
			return editTasks(taskFile, []string{id}, func(task *model.Task) {
				task.Status, task.Solo = model.StatusOpen, true
			})
		},
		ReleaseTasks: func(ids []string) error {
			return editTasks(taskFile, ids, func(task *model.Task) {
				if task.Status == model.StatusWIP {
					task.Status, task.StartedAt = model.StatusOpen, nil
				}
			})
		},
	}
	for _, mod := range mods {
		mod(opts)
	}
	go func() {
		res, err := RunDriver(opts)
		done <- outcome{res, err}
	}()
	select {
	case o := <-done:
		return o.res, o.err
	case <-time.After(90 * time.Second):
		t.Fatal("the driver loop did not terminate")
		return DriverResult{}, nil
	}
}

// assertBatchUnwound: both tasks are open and never solo, no worktree or
// tp-unit branch is left, and no unit was spawned.
func assertBatchUnwound(t *testing.T, root, taskFile, records string) {
	t.Helper()
	for _, id := range []string{"alpha", "beta"} {
		task := taskByID(t, taskFile, id)
		assert.Equal(t, model.StatusOpen, task.Status, "%s is released", id)
		assert.Nil(t, task.StartedAt, "%s carries no claim", id)
		assert.False(t, task.Solo, "%s is not marked solo", id)
	}
	assert.NotContains(t, gitIn(t, root, "worktree", "list"), "tp-worktrees", "every worktree is removed")
	assert.Empty(t, gitIn(t, root, "branch", "--list", unitBranchPrefix+"*"), "every tp-unit branch is removed")
	assert.Empty(t, invocations(t, records), "no unit is spawned, so no attempt is spent")
	st, err := ReadRunState(root, taskFile)
	require.NoError(t, err)
	assert.Empty(t, st.Units)
	assert.Zero(t, st.Totals.Units)
}

// TestRunDriver_WorktreeSetupRunsInEachWorktreeWithItsEnvironment: the setup
// runs once per batch worktree, there, with TP_MAIN_ROOT, TP_UNIT_ID and
// TP_UNATTENDED=1 on top of the driver's environment, and may write ignored
// files (spec/backlog/worktree-setup.md §1).
func TestRunDriver_WorktreeSetupRunsInEachWorktreeWithItsEnvironment(t *testing.T) {
	probe := t.TempDir()
	t.Setenv("TP_SETUP_PROBE", probe)
	root, spec, taskFile, _ := setupBatchProject(t,
		`mkdir -p cache && touch cache/warm && printf '%s\n%s\n%s\n%s\n' "$PWD" "$TP_MAIN_ROOT" "$TP_UNIT_ID" "$TP_UNATTENDED" > "$TP_SETUP_PROBE/$TP_UNIT_ID"; echo setup-output`)
	t.Setenv(fakerunner.EnvDurable, "1")

	res, err := driveSetupBatch(t, root, spec, taskFile)
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)

	for _, id := range []string{"alpha", "beta"} {
		data, readErr := os.ReadFile(filepath.Join(probe, id))
		require.NoError(t, readErr, "the setup ran for %s", id)
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		require.Len(t, lines, 4)
		assert.True(t, strings.HasPrefix(CanonicalPath(lines[0]), CanonicalPath(WorktreeBase(root))), "the setup runs in the batch worktree: %s", lines[0])
		assert.Equal(t, id, filepath.Base(lines[0]))
		assert.Equal(t, CanonicalPath(root), CanonicalPath(lines[1]), "TP_MAIN_ROOT is the main tree")
		assert.Equal(t, id, lines[2], "TP_UNIT_ID is the task")
		assert.Equal(t, "1", lines[3], "TP_UNATTENDED is set")
	}

	st, err := ReadRunState(root, taskFile)
	require.NoError(t, err)
	require.Len(t, st.Setups, 2, "each setup is recorded beside the units")
	for _, row := range st.Setups {
		require.NotNil(t, row.ExitCode)
		assert.Equal(t, 0, *row.ExitCode)
		log, readErr := os.ReadFile(row.LogPath)
		require.NoError(t, readErr)
		assert.Contains(t, string(log), "setup-output", "the command's output goes to the log")
		assert.Contains(t, string(log), "wall time", "the log records the setup's wall time")
	}
}

// TestRunDriver_AFailingWorktreeSetupUnwindsTheBatch: a setup that exits
// non-zero releases every claimed task, removes every worktree and branch,
// spends no attempt, and stops the run with worktree-setup-failed naming the
// task and its log.
func TestRunDriver_AFailingWorktreeSetupUnwindsTheBatch(t *testing.T) {
	root, spec, taskFile, records := setupBatchProject(t, `echo refusing; test "$TP_UNIT_ID" != beta`)

	res, err := driveSetupBatch(t, root, spec, taskFile)
	require.NoError(t, err, "a failed setup is a stop, not a driver error")
	assert.Equal(t, StopWorktreeSetupFailed, res.StopReason)
	require.NotNil(t, res.WorktreeSetup)
	assert.Equal(t, "beta", res.WorktreeSetup.ID)
	log, readErr := os.ReadFile(res.WorktreeSetup.LogPath)
	require.NoError(t, readErr, "the stop names the setup's log")
	assert.Contains(t, string(log), "refusing")

	assertBatchUnwound(t, root, taskFile, records)
	st, err := ReadRunState(root, taskFile)
	require.NoError(t, err)
	require.NotNil(t, st.StopReason)
	assert.Equal(t, StopWorktreeSetupFailed, *st.StopReason)
}

// TestRunDriver_AnUntrackedFileFailsTheWorktreeSetup: only ignored files may
// be written; an untracked one would ride into the unit's commits.
func TestRunDriver_AnUntrackedFileFailsTheWorktreeSetup(t *testing.T) {
	root, spec, taskFile, records := setupBatchProject(t, `touch stray.txt`)

	res, err := driveSetupBatch(t, root, spec, taskFile)
	require.NoError(t, err)
	assert.Equal(t, StopWorktreeSetupFailed, res.StopReason)
	require.NotNil(t, res.WorktreeSetup)
	assert.Equal(t, "alpha", res.WorktreeSetup.ID, "the first setup in batch order fails")
	log, readErr := os.ReadFile(res.WorktreeSetup.LogPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(log), "stray.txt", "the log names what dirtied the tree")
	assertBatchUnwound(t, root, taskFile, records)
}

// TestRunDriver_AFailedWorktreeAddUnwindsTheBatch: a git worktree add that
// fails after an earlier one succeeded releases the batch the same way, with
// no setup configured. A post-checkout hook refusing beta's checkout leaves
// beta's own worktree and branch behind, which the unwind removes too.
func TestRunDriver_AFailedWorktreeAddUnwindsTheBatch(t *testing.T) {
	root, spec, taskFile, records := setupBatchProject(t, "")
	hook := filepath.Join(root, ".git", "hooks", "post-checkout")
	require.NoError(t, os.MkdirAll(filepath.Dir(hook), 0o750))
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\ncase \"$PWD\" in */beta) exit 1;; esac\nexit 0\n"), 0o700)) //nolint:gosec // a hook must be executable

	res, err := driveSetupBatch(t, root, spec, taskFile)
	var driverErr *DriverError
	require.True(t, errors.As(err, &driverErr), "a failed add stays a driver error: %v", err)
	assert.Equal(t, StopDriverError, res.StopReason)
	assertBatchUnwound(t, root, taskFile, records)
}

// TestRunDriver_UnsetWorktreeSetupRecordsNothing: with no command the batch
// runs as before and the run state carries no setups key.
func TestRunDriver_UnsetWorktreeSetupRecordsNothing(t *testing.T) {
	root, spec, taskFile, _ := setupBatchProject(t, "")
	t.Setenv(fakerunner.EnvDurable, "1")

	res, err := driveSetupBatch(t, root, spec, taskFile)
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	data, err := os.ReadFile(RunStatePath(root, taskFile))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "setups")
	logs, err := filepath.Glob(filepath.Join(RunDir(root, res.RunID), "setup-*.log"))
	require.NoError(t, err)
	assert.Empty(t, logs)
}

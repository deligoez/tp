package engine

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// The four points a driver can die at inside a rolling pool
// (spec/backlog/rolling-implement.md §2). Each test ends the driver's
// goroutine at its point with runtime.Goexit, which runs no more of the loop
// and leaves the run state, the worktrees, the claims and the pending closes as
// a SIGKILL would; the children it spawned run on, as they would. A fresh run
// over the same cycle must then finish it.

// killAt arms the pool hook to end the driver at point for task id.
func killAt(t *testing.T, point, id string) {
	t.Helper()
	poolHook = func(p, i string) {
		if p == point && i == id {
			runtime.Goexit()
		}
	}
	t.Cleanup(disarm)
}

func disarm() { poolHook = func(string, string) {} }

// driveUntilKilled runs the loop and requires it to die before it returns,
// then waits for every child it spawned to exit.
func driveUntilKilled(t *testing.T, records string, opts *DriverOptions) {
	t.Helper()
	returned := make(chan bool, 1)
	go func() {
		finished := false
		defer func() { returned <- finished }()
		_, _ = RunDriver(opts)
		finished = true
	}()
	select {
	case finished := <-returned:
		require.False(t, finished, "the driver reached its kill point")
	case <-time.After(60 * time.Second):
		t.Fatal("the driver neither died nor returned")
	}
	waitForChildren(t, records)
}

// waitForChildren waits until every child a driver spawned has exited.
func waitForChildren(t *testing.T, records string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return claimedInvocations(t, records) == len(invocations(t, records))
	}, 30*time.Second, 20*time.Millisecond, "the driver's children exit")
}

// assertRecovered: a fresh run converges with every task done once, and the
// cycle is left with no worktree, no pending close and no unit branch that
// still holds work.
func assertRecovered(t *testing.T, root, spec, taskFile string, ids ...string) {
	t.Helper()
	disarm()
	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	subjects := strings.Split(gitIn(t, root, "log", "--format=%s"), "\n")
	for _, id := range ids {
		assert.Equal(t, model.StatusDone, taskByID(t, taskFile, id).Status, "%s is done", id)
		// A unit resumed in the main tree closes without a unit commit.
		assert.LessOrEqual(t, countOf(subjects, "unit "+id), 1, "%s's commit is never picked twice", id)
	}
	pending, err := ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	assert.Empty(t, pending, "no request is left")
	assert.NotContains(t, gitIn(t, root, "worktree", "list"), "tp-worktrees", "no worktree is left")
	assert.Empty(t, CycleUnitBranches(root, taskFile), "no unit branch holds unfinished work")
}

func countOf(lines []string, want string) int {
	n := 0
	for _, l := range lines {
		if l == want {
			n++
		}
	}
	return n
}

// TestCrash_DuringAPick: the driver dies with alpha's commits picked onto
// the main branch and its close not yet run. The next run finds the whole pick
// landed and closes it, without picking it twice.
func TestCrash_DuringAPick(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")
	killAt(t, "picked", "alpha")

	driveUntilKilled(t, records, rollingOptions(root, spec, taskFile, driverWorkflow()))
	assert.Equal(t, model.StatusWIP, taskByID(t, taskFile, "alpha").Status, "alpha died before its close")
	assertRecovered(t, root, spec, taskFile, "alpha", "beta")
	assert.Equal(t, 1, countOf(strings.Split(gitIn(t, root, "log", "--format=%s"), "\n"), "unit alpha"), "the landed pick is closed, not dropped")
}

// TestCrash_DuringAPickOfALaterID: the driver dies with beta picked and not
// closed while alpha still runs; alpha's close then lands on disk too. The
// next run must recover beta's interrupted pick before it picks alpha on top
// of it, or beta's recorded start no longer matches what landed since.
func TestCrash_DuringAPickOfALaterID(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=1500,beta=100")
	killAt(t, "picked", "beta")

	driveUntilKilled(t, records, rollingOptions(root, spec, taskFile, driverWorkflow()))
	assert.True(t, HasPendingClose(root, taskFile, "alpha"), "alpha asked to close after the driver died")
	assertRecovered(t, root, spec, taskFile, "alpha", "beta")
}

// TestCrash_AHaltedPoolsRefusedPickGoesFirstOnTheNextRun: beta's close is
// refused as a driver error, which halts the pool with beta's pick start
// recorded, and alpha's close is left for the next run unintegrated. The next
// run must pick beta first: alpha, picked first by id, would land under beta's
// recorded start.
func TestCrash_AHaltedPoolsRefusedPickGoesFirstOnTheNextRun(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=1500,beta=100")
	opts := rollingOptions(root, spec, taskFile, driverWorkflow())
	closeTask := opts.CloseTask
	opts.CloseTask = func(id, reason string, picked []string, logPath string) error {
		if id == "beta" {
			return &CloseRefusedError{ID: id, LogPath: logPath}
		}
		return closeTask(id, reason, picked, logPath)
	}

	res, err := RunDriver(opts)
	require.Error(t, err)
	assert.Equal(t, StopDriverError, res.StopReason)
	waitForChildren(t, records)
	assert.True(t, HasPendingClose(root, taskFile, "alpha"), "the halted pool left alpha's close unintegrated")
	assertRecovered(t, root, spec, taskFile, "alpha", "beta")
}

// TestCrash_AfterACloseBeforeItsRequestIsDeleted: alpha is done on the main
// branch and its request is still on disk. The next run drops the request.
func TestCrash_AfterACloseBeforeItsRequestIsDeleted(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")
	killAt(t, "closed", "alpha")

	driveUntilKilled(t, records, rollingOptions(root, spec, taskFile, driverWorkflow()))
	assert.Equal(t, model.StatusDone, taskByID(t, taskFile, "alpha").Status)
	assert.True(t, HasPendingClose(root, taskFile, "alpha"), "the request outlived the close")
	assertRecovered(t, root, spec, taskFile, "alpha", "beta")
}

// TestCrash_DuringAWorktreeSetup: the driver dies inside beta's setup, beta
// claimed and its worktree added, alpha already running. The next run removes
// beta's worktree and resumes it as the wip task it is.
func TestCrash_DuringAWorktreeSetup(t *testing.T) {
	root, spec, taskFile, records := setupBatchProject(t, "true")
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")
	opts := rollingOptions(root, spec, taskFile, driverWorkflow())
	opts.runSetup = func(ctx context.Context, command, dir string, env []string, timeout time.Duration, logPath string) (*int, string) {
		if slices.Contains(env, EnvUnitID+"=beta") {
			runtime.Goexit()
		}
		return runSetupCommand(ctx, command, dir, env, timeout, logPath)
	}

	driveUntilKilled(t, records, opts)
	assert.Equal(t, []string{"alpha"}, spawnedIDs(invocations(t, records)), "beta died before its spawn")
	assert.Equal(t, model.StatusWIP, taskByID(t, taskFile, "beta").Status)
	assertRecovered(t, root, spec, taskFile, "alpha", "beta")
}

// TestCrash_DuringARefillAfterAWorktreeAdd: the driver dies refilling beta's
// slot, gamma's worktree added and no child spawned, while alpha still runs.
func TestCrash_DuringARefillAfterAWorktreeAdd(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta", "gamma"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=1500,beta=100,gamma=100")
	killAt(t, "added", "gamma")

	driveUntilKilled(t, records, rollingOptions(root, spec, taskFile, driverWorkflow()))
	assert.Equal(t, []string{"alpha", "beta"}, spawnedIDs(invocations(t, records)), "gamma died before its spawn")
	assert.Equal(t, model.StatusWIP, taskByID(t, taskFile, "gamma").Status)
	assertRecovered(t, root, spec, taskFile, "alpha", "beta", "gamma")
}

//go:build unix

package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// TestUnitLock_ARetryStartsWhileALeftoverProcessHoldsTheLock: a process the
// unit's last attempt left behind still holds its lock. The retry is not a
// driver error: it starts, without the lock.
func TestUnitLock_ARetryStartsWhileALeftoverProcessHoldsTheLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locks", "run-k-unit-alpha.lock")
	held, err := lockUnitFile(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })

	a := &unitAttempt{
		unit:     NextUnit{Kind: UnitImplement, ID: "alpha"},
		runner:   &Runner{Cmd: "true"},
		logPath:  filepath.Join(dir, "alpha.log"),
		lockPath: path,
		dir:      dir,
	}
	require.True(t, a.start(dir, nil), "the retry starts")
	a.wait()
	assert.NoError(t, a.err)
	assert.Equal(t, 0, a.exitCode)
}

// leftoverWait bounds how long a run that must not wait may take: well under
// the LeaveBehindSeconds a leftover process holds the lock for.
const leftoverWait = 4 * time.Second

// TestUnitLock_ACleanStopsLeftoverProcessIsNotWaitedFor: both units leave a
// background process holding their lock, and the run stops cleanly with both
// tasks integrated. The next run does not wait for those processes: no unit
// of a cleanly stopped run is live.
func TestUnitLock_ACleanStopsLeftoverProcessIsNotWaitedFor(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")
	t.Setenv(fakerunner.EnvLeaveBehindIDs, "alpha,beta")

	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	require.Equal(t, StopConverged, res.StopReason)
	require.True(t, unitLockHeld(unitLockPath(taskFile, "alpha")), "a leftover process holds alpha's lock")

	start := time.Now()
	res, err = driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	assert.Less(t, time.Since(start), leftoverWait, "the next run does not wait for the leftover processes")
}

// TestUnitLock_ACrashedRunsDeadChildIsNotWaitedForThoughItsLeftoverHoldsTheLock:
// the driver dies after alpha's child exited, and the process alpha left
// behind still holds its lock. Alpha's child is dead, so the next run does
// not wait: it integrates alpha's close at once.
func TestUnitLock_ACrashedRunsDeadChildIsNotWaitedForThoughItsLeftoverHoldsTheLock(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")
	t.Setenv(fakerunner.EnvLeaveBehindIDs, "alpha")
	killAt(t, "picked", "alpha")

	driveUntilKilled(t, records, rollingOptions(root, spec, taskFile, driverWorkflow()))
	require.True(t, unitLockHeld(unitLockPath(taskFile, "alpha")), "alpha's leftover process holds its lock")

	start := time.Now()
	assertRecovered(t, root, spec, taskFile, "alpha", "beta")
	assert.Less(t, time.Since(start), leftoverWait, "the next run does not wait for a dead child's leftover")
}

// deadRunState writes a run state a dead run left: alpha in the pool, its
// child's pid on its row, no stop reason, and no run lock held.
func deadRunState(t *testing.T, root, taskFile string, pid int) {
	t.Helper()
	rec, err := NewRunRecorder(root, taskFile, "dead", "implement")
	require.NoError(t, err)
	require.NoError(t, rec.JoinPool("alpha"))
	require.NoError(t, rec.StartUnit(&RunUnitRow{Seq: 1, Kind: UnitImplement, ID: "alpha", Attempt: 1, Worktree: t.TempDir()}))
	require.NoError(t, rec.SetUnitPID(1, pid))
}

// exitedPID is the pid of a process that has exited and been reaped.
func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	require.NoError(t, cmd.Run())
	return cmd.Process.Pid
}

// TestLiveRunningUnits_ADeadRunsLiveChildIsRunningForTheReader: the run lock
// is free, but the dead run's alpha is in its pool, its child is alive and its
// lock is held. tp resume's reader names it running, as a restarted run would
// wait for it; with the child gone, though the lock is still held, it does
// not.
func TestLiveRunningUnits_ADeadRunsLiveChildIsRunningForTheReader(t *testing.T) {
	root, _, taskFile, _ := batchProject(t, rollingTasks(2, "alpha"))
	held, err := lockUnitFile(unitLockPath(taskFile, "alpha"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })

	deadRunState(t, root, taskFile, os.Getpid())
	assert.Equal(t, []string{"alpha"}, LiveRunningUnits(root, taskFile), "a live child of a dead run is running")

	deadRunState(t, root, taskFile, exitedPID(t))
	assert.Empty(t, LiveRunningUnits(root, taskFile), "a held lock whose unit's child exited is not")
}

// TestUnitLock_AWaitingRunCarriesTheOrphanInItsPool: while a restarted run
// waits for the dead run's live alpha, its own fresh run state lists alpha in
// its pool, so an outside reader names alpha running rather than offering
// its task; once alpha's close is recovered, the pool is empty again.
func TestUnitLock_AWaitingRunCarriesTheOrphanInItsPool(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, rollingTasks(2, "alpha"))
	held, err := lockUnitFile(unitLockPath(taskFile, "alpha"))
	require.NoError(t, err)
	rec, err := NewRunRecorder(root, taskFile, "r2", "")
	require.NoError(t, err)
	d := &driver{
		opts:    &DriverOptions{Root: root, TaskFile: taskFile, Spec: spec},
		runID:   "r2",
		rec:     rec,
		sig:     &interrupts{ch: make(chan os.Signal, 1)},
		orphans: []liveUnit{{ID: "alpha", PID: os.Getpid()}},
	}
	var during []string
	poolHook = func(point, _ string) {
		if point == "waiting" {
			during = LiveRunningUnits(root, taskFile)
			_ = held.Close() // alpha exits
		}
	}
	t.Cleanup(disarm)

	require.NoError(t, d.reconcileUnits())
	assert.Equal(t, []string{"alpha"}, during, "the waiting run's pool names the orphan")
	assert.Empty(t, poolSeenOnDisk(t, root, taskFile), "the orphan leaves the pool once recovered")
}

// TestUnitLock_ASignalDuringTheRecoveryWaitTouchesNothing: a dead run's unit
// still holds its lock and the operator's signal is already in. Recovery
// returns at once without integrating the unit's pending close, so the
// loop's checkpoint, which reads the signal first, stops the run.
func TestUnitLock_ASignalDuringTheRecoveryWaitTouchesNothing(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, `{"spec":"s.md","tasks":[{"id":"alpha","title":"A","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"],"touches":["alpha.txt"]}]}`)
	require.NoError(t, WritePendingClose(root, taskFile, &PendingClose{ID: "alpha", Commits: []string{"abc"}}))
	held, err := lockUnitFile(unitLockPath(taskFile, "alpha"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })
	rec, err := NewRunRecorder(root, taskFile, "r1", "")
	require.NoError(t, err)
	d := &driver{
		opts:    &DriverOptions{Root: root, TaskFile: taskFile, Spec: spec},
		runID:   "r1",
		rec:     rec,
		sig:     &interrupts{ch: make(chan os.Signal, 1), seen: true},
		orphans: []liveUnit{{ID: "alpha", PID: os.Getpid()}},
	}

	done := make(chan error, 1)
	go func() { done <- d.reconcileUnits() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the signal did not cut the wait short")
	}
	assert.True(t, HasPendingClose(root, taskFile, "alpha"), "the live unit's pending close is left as it is")
	assert.True(t, unitLockHeld(unitLockPath(taskFile, "alpha")))
}

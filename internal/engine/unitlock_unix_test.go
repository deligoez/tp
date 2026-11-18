//go:build unix

package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	d := &driver{
		opts:  &DriverOptions{Root: root, TaskFile: taskFile, Spec: spec},
		runID: "r1",
		sig:   &interrupts{ch: make(chan os.Signal, 1), seen: true},
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
	assert.Equal(t, []string{"alpha"}, heldUnitLocks(taskFile))
}

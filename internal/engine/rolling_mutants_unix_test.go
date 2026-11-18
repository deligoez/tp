//go:build unix

package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnitLock_TheLockIsFreeOnceTheChildExits: the driver hands the child its
// unit lock and closes its own copy, so once the child exits nobody holds it,
// though the driver lives on.
func TestUnitLock_TheLockIsFreeOnceTheChildExits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locks", "run-k-unit-alpha.lock")
	a := &unitAttempt{
		unit:     NextUnit{Kind: UnitImplement, ID: "alpha"},
		runner:   &Runner{Cmd: "true"},
		logPath:  filepath.Join(dir, "alpha.log"),
		lockPath: path,
		dir:      dir,
	}
	require.True(t, a.start(dir, nil))
	a.wait()
	require.NoError(t, a.err)
	assert.False(t, unitLockHeld(path), "the driver's own copy of the lock was closed")
}

// TestLiveUnits_APoolMemberWithoutAPidIsNotLive: the dead run's alpha is in
// its pool and its lock is held, but no pid was ever recorded for it. Without
// its own child's pid it is not live; pid 0 is not a process.
func TestLiveUnits_APoolMemberWithoutAPidIsNotLive(t *testing.T) {
	root, _, taskFile := setupResumeProject(t, rollingTasks(2, "alpha"))
	held, err := lockUnitFile(unitLockPath(taskFile, "alpha"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })

	deadRunState(t, root, taskFile, 0)
	assert.Empty(t, LiveRunningUnits(root, taskFile))
}

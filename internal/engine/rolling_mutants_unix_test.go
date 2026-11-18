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


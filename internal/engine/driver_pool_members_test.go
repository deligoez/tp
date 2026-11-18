package engine

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// poolSeenOnDisk reads the run state the driver wrote and returns the pool it
// names, the reader's view without the lock probe.
func poolSeenOnDisk(t *testing.T, root, taskFile string) []string {
	t.Helper()
	st, err := ReadRunState(root, taskFile)
	require.NoError(t, err)
	return poolMembers(st)
}

// TestPoolMembers_AUnitIsAMemberFromItsClaimThroughItsSetup: while beta's
// worktree is being set up, beta is claimed and wip but has no row yet; a
// reader must still see it in the pool, or it offers that wip task.
func TestPoolMembers_AUnitIsAMemberFromItsClaimThroughItsSetup(t *testing.T) {
	root, spec, taskFile, _ := setupBatchProject(t, "true")
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")
	var seen []string
	opts := rollingOptions(root, spec, taskFile, driverWorkflow())
	opts.runSetup = func(ctx context.Context, command, dir string, env []string, timeout time.Duration, logPath string) (*int, string) {
		if slices.Contains(env, EnvUnitID+"=beta") {
			seen = poolSeenOnDisk(t, root, taskFile)
		}
		return runSetupCommand(ctx, command, dir, env, timeout, logPath)
	}

	res, err := RunDriver(opts)
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	assert.Equal(t, []string{"alpha", "beta"}, seen, "beta is a member during its setup")
	assert.Empty(t, poolSeenOnDisk(t, root, taskFile), "an emptied pool names no one")
}

// TestPoolMembers_AUnitIsAMemberUntilItIsIntegrated: beta has exited and its
// row is closed while its close is still being picked and gated on the main
// branch; a reader must still see it in the pool.
func TestPoolMembers_AUnitIsAMemberUntilItIsIntegrated(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=1500,beta=100")
	var mu sync.Mutex
	seen := map[string][]string{}

	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow(), func(o *DriverOptions) {
		inner := o.CloseTask
		o.CloseTask = func(id, reason string, commits []string, logPath string) error {
			mu.Lock()
			seen[id] = poolSeenOnDisk(t, root, taskFile)
			mu.Unlock()
			return inner(id, reason, commits, logPath)
		}
	})
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	assert.Equal(t, []string{"alpha", "beta"}, seen["beta"], "beta is a member while it is integrated")
	assert.Equal(t, []string{"alpha"}, seen["alpha"], "alpha alone is left by its own close")
}

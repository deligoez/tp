package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// TestPool_ADriverErrorAfterASetupFailureIsTheStop: beta's setup fails, which
// drains the pool; alpha, still running, then fails to integrate as a driver
// error. The driver error is the more severe and is the stop; the setup
// failure is still named.
func TestPool_ADriverErrorAfterASetupFailureIsTheStop(t *testing.T) {
	root, spec, taskFile, _ := setupBatchProject(t, `test "$TP_UNIT_ID" != beta`)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=800,beta=100")
	opts := rollingOptions(root, spec, taskFile, driverWorkflow())
	opts.CloseTask = func(id, _ string, _ []string, logPath string) error {
		return &CloseRefusedError{ID: id, LogPath: logPath}
	}

	res, err := RunDriver(opts)
	require.Error(t, err)
	assert.Equal(t, StopDriverError, res.StopReason, "the integration's driver error outranks the setup failure")
	require.NotNil(t, res.WorktreeSetup, "the setup failure is still named")
	assert.Equal(t, "beta", res.WorktreeSetup.ID)
}

// TestPool_TwoEscalationsRecordTheLowestSeq: alpha (seq 1) and beta (seq 2)
// both escalate, and beta exits first. The run records alpha's, the lowest
// seq, not the first to arrive.
func TestPool_TwoEscalationsRecordTheLowestSeq(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvEscalateIDs, "alpha,beta")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=800,beta=100")

	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	assert.Equal(t, StopEscalation, res.StopReason)
	assert.Equal(t, filepath.Base(EscalationPath("", "1")), filepath.Base(res.EscalationPath), "alpha's record, seq 1, though beta's arrived first")
}

// TestPool_ASetupThatCrossesTheWallClockCapSpawnsNothing: alpha's setup is
// still running when the wall-clock cap is reached. The caps are read again
// before the spawn, so alpha is released unspawned, open and out of the pool,
// and the run stops on the cap.
func TestPool_ASetupThatCrossesTheWallClockCapSpawnsNothing(t *testing.T) {
	root, spec, taskFile, records := setupBatchProject(t, "true")
	t.Setenv(fakerunner.EnvDurable, "1")
	wf := driverWorkflow()
	wf.RunMaxWallClockSeconds = 1
	opts := rollingOptions(root, spec, taskFile, wf)
	opts.runSetup = func(ctx context.Context, command, dir string, env []string, timeout time.Duration, logPath string) (*int, string) {
		time.Sleep(1200 * time.Millisecond)
		return runSetupCommand(ctx, command, dir, env, timeout, logPath)
	}

	res, err := RunDriver(opts)
	require.NoError(t, err)
	assert.Equal(t, StopCapWallClock, res.StopReason)
	assert.Empty(t, spawnedIDs(invocations(t, records)), "nothing is spawned past the cap")
	assert.Equal(t, model.StatusOpen, taskByID(t, taskFile, "alpha").Status, "alpha goes back to open")
	assert.NotContains(t, gitIn(t, root, "worktree", "list"), "tp-worktrees", "alpha's worktree is removed")
	assert.Empty(t, poolSeenOnDisk(t, root, taskFile))
}

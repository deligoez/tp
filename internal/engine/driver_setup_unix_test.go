//go:build unix

package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunDriver_ATimedOutWorktreeSetupKillsItsProcessGroup: a setup that
// backgrounds a child and outlives its timeout is killed with everything it
// started, and the batch is unwound with no worktree left behind. The gate
// timeout's 30 s floor is too long for a test, so the runner is handed a
// short one.
func TestRunDriver_ATimedOutWorktreeSetupKillsItsProcessGroup(t *testing.T) {
	probe := t.TempDir()
	t.Setenv("TP_SETUP_PROBE", probe)
	root, spec, taskFile, records := setupBatchProject(t,
		`sleep 30 & echo $! > "$TP_SETUP_PROBE/child"; wait`)

	start := time.Now()
	res, err := driveSetupBatch(t, root, spec, taskFile, func(o *DriverOptions) {
		o.runSetup = func(ctx context.Context, command, dir string, env []string, _ time.Duration, logPath string) (*int, string) {
			return runSetupCommand(ctx, command, dir, env, 500*time.Millisecond, logPath)
		}
	})
	require.NoError(t, err)
	assert.Equal(t, StopWorktreeSetupFailed, res.StopReason)
	assert.Less(t, time.Since(start), 20*time.Second, "the run stops at the timeout")

	data, err := os.ReadFile(filepath.Join(probe, "child"))
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	assert.Eventually(t, func() bool { return syscall.Kill(pid, 0) != nil },
		5*time.Second, 50*time.Millisecond, "the backgrounded child dies with its setup")

	assertBatchUnwound(t, root, taskFile, records)
	left, err := filepath.Glob(filepath.Join(WorktreeBase(root), "*", "*", "alpha"))
	require.NoError(t, err)
	assert.Empty(t, left, "the timed-out worktree is gone from disk")
}

// TestRunDriver_AWorktreeTheUnwindCannotRemoveIsADriverError: a failed setup
// that left a directory nobody may empty keeps its worktree on disk through
// the unwind, and the driver reports that instead of passing over it.
func TestRunDriver_AWorktreeTheUnwindCannotRemoveIsADriverError(t *testing.T) {
	probe := t.TempDir()
	t.Setenv("TP_SETUP_PROBE", probe)
	root, spec, taskFile, _ := setupBatchProject(t,
		`mkdir -p cache/locked && touch cache/locked/f && chmod 500 cache/locked && echo "$PWD/cache/locked" > "$TP_SETUP_PROBE/locked"; exit 1`)
	t.Cleanup(func() {
		if data, err := os.ReadFile(filepath.Join(probe, "locked")); err == nil {
			_ = os.Chmod(strings.TrimSpace(string(data)), 0o700) //nolint:gosec // the test's own directory, made removable again
		}
		_ = os.RemoveAll(WorktreeBase(root))
	})

	res, err := driveSetupBatch(t, root, spec, taskFile)
	var driverErr *DriverError
	require.ErrorAs(t, err, &driverErr)
	assert.Contains(t, err.Error(), "still on disk")
	assert.Equal(t, StopDriverError, res.StopReason)
}

package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// startedTP is a tp child started in the background, for a test that acts on
// it while it waits for a gate slot.
type startedTP struct {
	cmd            *exec.Cmd
	stdout, stderr bytes.Buffer
	done           chan error
}

func startTP(t *testing.T, dir string, extra []string, args ...string) *startedTP {
	t.Helper()
	s := &startedTP{done: make(chan error, 1)}
	s.cmd = exec.Command(binaryPath, append([]string{"--json"}, args...)...)
	s.cmd.Dir = dir
	s.cmd.Env = append(append(byHandEnv(), "NO_COLOR=1", "TP_HC=0"), extra...)
	s.cmd.Stdout, s.cmd.Stderr = &s.stdout, &s.stderr
	require.NoError(t, s.cmd.Start())
	go func() { s.done <- s.cmd.Wait() }()
	t.Cleanup(func() { _ = s.cmd.Process.Kill() })
	return s
}

// exited waits up to d for the child and reports its exit code; false when it
// is still running.
func (s *startedTP) exited(t *testing.T, d time.Duration) (code int, ok bool) {
	t.Helper()
	select {
	case err := <-s.done:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), true
		}
		require.NoError(t, err)
		return 0, true
	case <-time.After(d):
		return 0, false
	}
}

// holdSlots takes every one of capN slots under dir for the test, as gates
// already running would.
func holdSlots(t *testing.T, dir string, capN int) []*engine.GateSlot {
	t.Helper()
	slots := make([]*engine.GateSlot, 0, capN)
	for range capN {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		slot, err := engine.AcquireGateSlot(ctx, dir, capN, false)
		cancel()
		require.NoError(t, err)
		slots = append(slots, slot)
	}
	return slots
}

// capGates sets the project's gate cap, keeping its gate command.
func capGates(t *testing.T, root, extra string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"quality_gate":"touch gate-ran","run_max_parallel_gates":1`+extra+`}}`), 0o600))
}

// TestDone_AReusedMainGateTakesNoSlot: with the only slot held, a main-branch
// close that reuses the unit's gate goes through, and one that must run the
// gate waits for the slot and runs it only once the slot is free.
func TestDone_AReusedMainGateTakesNoSlot(t *testing.T) {
	t.Parallel()
	slots := filepath.Join(t.TempDir(), "gate-slots")
	held := holdSlots(t, slots, 1)

	root, taskFile, picked := pickedPendingClose(t)
	capGates(t, root, "")
	env := []string{"TP_FILE=" + taskFile, engine.EnvGateMainSlots + "=" + slots}
	reused := startTP(t, root, env, "done", "t1", "implemented and verified", "--commit", picked)
	code, ok := reused.exited(t, 10*time.Second)
	require.True(t, ok, "a reused gate does not wait for the held slot: %s", reused.stderr.String())
	require.Equal(t, 0, code, reused.stderr.String())
	assert.Contains(t, reused.stderr.String(), "gate reused from the unit's worktree")

	root, taskFile, picked = pickedPendingClose(t)
	capGates(t, root, "")
	require.NoError(t, os.WriteFile(filepath.Join(root, "kept.go"), []byte("package a\n"), 0o600))
	env = []string{"TP_FILE=" + taskFile, engine.EnvGateMainSlots + "=" + slots}
	gated := startTP(t, root, env, "done", "t1", "implemented and verified", "--commit", picked)
	_, ok = gated.exited(t, time.Second)
	// The child is still writing its output, so its buffers are read only
	// once it has exited.
	require.False(t, ok, "a gate that must run waits for the held slot")
	assert.NoFileExists(t, filepath.Join(root, "gate-ran"), "and does not run while it waits")

	held[0].Release()
	code, ok = gated.exited(t, 10*time.Second)
	require.True(t, ok, "the freed slot lets the gate run")
	require.Equal(t, 0, code, gated.stderr.String())
	assert.FileExists(t, filepath.Join(root, "gate-ran"))
}

// runUnit is a claimed task's committed change in a unit's worktree, under a
// project capping its gates at one, with the environment tp run gives the
// unit: its run directory and attempt seq. slots is where that run's slots are.
type runUnit struct {
	root, worktree, taskFile, sha, runDir, slots string
	env                                          []string
}

func unitInRun(t *testing.T, extraConfig string) runUnit {
	t.Helper()
	var u runUnit
	u.root, u.worktree, u.taskFile = pendingProject(t)
	capGates(t, u.root, extraConfig)
	require.NoError(t, os.WriteFile(filepath.Join(u.worktree, "a.go"), []byte("package a\n"), 0o600))
	gitRun(t, u.worktree, "add", "a.go")
	gitRun(t, u.worktree, "commit", "-q", "-m", "add a")
	u.sha = gitRun(t, u.worktree, "rev-parse", "HEAD")
	u.runDir = engine.RunDir(u.root, "01JTESTRUN0000000000000000")
	u.slots = engine.GateSlotsDir(u.runDir)
	u.env = []string{"TP_FILE=" + u.taskFile, engine.EnvRunDir + "=" + u.runDir, engine.EnvUnitSeq + "=1"}
	return u
}

// TestDone_AWaitLongerThanTheGateTimeoutSpendsNothing: a unit that waits for
// a slot longer than gate_timeout_seconds still gets its whole gate, since the
// timeout starts once the slot is taken, and its close is asked for as if it
// had never waited. The driver has no unit or attempt timeout, so nothing else
// can kill it or spend its attempt meanwhile.
func TestDone_AWaitLongerThanTheGateTimeoutSpendsNothing(t *testing.T) {
	t.Parallel()
	u := unitInRun(t, `,"gate_timeout_seconds":30`)
	held := holdSlots(t, u.slots, 1)
	time.AfterFunc(32*time.Second, held[0].Release)

	start := time.Now()
	unit := startTP(t, u.worktree, u.env, "done", "t1", "implemented and verified", "--commit", u.sha)
	code, ok := unit.exited(t, 60*time.Second)
	require.True(t, ok, "the unit finishes once the slot is free")
	require.Equal(t, 0, code, unit.stderr.String())
	assert.GreaterOrEqual(t, time.Since(start), 30*time.Second, "the wait outlasted the gate timeout")
	assert.FileExists(t, filepath.Join(u.worktree, "gate-ran"), "the gate ran to the end after the wait")

	pending, err := engine.ReadPendingCloses(u.root, u.taskFile)
	require.NoError(t, err)
	require.Len(t, pending, 1, "the close is asked for: the attempt is not spent")
	gt, ok := engine.ReadGateTimes(u.runDir, 1)
	require.True(t, ok)
	assert.GreaterOrEqual(t, gt.WaitedSeconds, 30.0, "the wait is recorded apart from the gate")
	assert.Empty(t, gt.WaitingSince, "the wait is over")
}

// TestDone_ASignalWhileWaitingAsksForNoClose: a unit signalled while it waits
// for a slot exits on its own terms, having run no gate, asked for no close
// and taken no slot.
func TestDone_ASignalWhileWaitingAsksForNoClose(t *testing.T) {
	t.Parallel()
	u := unitInRun(t, "")
	held := holdSlots(t, u.slots, 1)

	unit := startTP(t, u.worktree, u.env, "done", "t1", "implemented and verified", "--commit", u.sha)
	require.Eventually(t, func() bool {
		gt, ok := engine.ReadGateTimes(u.runDir, 1)
		return ok && gt.WaitingSince != ""
	}, 10*time.Second, 20*time.Millisecond, "the unit waits for the slot")
	require.NoError(t, unit.cmd.Process.Signal(syscall.SIGTERM))

	code, ok := unit.exited(t, 10*time.Second)
	require.True(t, ok, "the signal ends the wait")
	assert.Equal(t, 4, code, "an exit of tp's own, not a death by the signal: %s", unit.stderr.String())
	assert.Contains(t, unit.stderr.String(), "interrupted while waiting for a gate slot")
	assert.NoFileExists(t, filepath.Join(u.worktree, "gate-ran"), "no gate ran")
	pending, err := engine.ReadPendingCloses(u.root, u.taskFile)
	require.NoError(t, err)
	assert.Empty(t, pending, "no close was asked for")

	held[0].Release()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	slot, err := engine.AcquireGateSlot(ctx, u.slots, 1, true)
	require.NoError(t, err, "the unit left no slot and no main claim behind")
	slot.Release()
}

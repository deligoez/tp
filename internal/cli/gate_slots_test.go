package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	require.False(t, ok, "a gate that must run waits for the held slot: %s", gated.stderr.String())
	assert.NoFileExists(t, filepath.Join(root, "gate-ran"), "and does not run while it waits")

	held[0].Release()
	code, ok = gated.exited(t, 10*time.Second)
	require.True(t, ok, "the freed slot lets the gate run")
	require.Equal(t, 0, code, gated.stderr.String())
	assert.FileExists(t, filepath.Join(root, "gate-ran"))
}

package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// gateConfig sets the project's gate command under a cap of one slot.
func gateConfig(t *testing.T, root, gate, extra string) {
	t.Helper()
	quoted, err := json.Marshal(gate)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"quality_gate":`+string(quoted)+`,"run_max_parallel_gates":1`+extra+`}}`), 0o600))
}

// projectGated is a project whose only gate is the project layer's, capped at
// one slot: no task-level override hides it.
func projectGated(t *testing.T, gate string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Test Spec\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	gateConfig(t, dir, gate, "")
	return dir
}

// TestGate_ABackgroundedChildDoesNotHoldTheSlot: a gate whose command exits
// while a child it backgrounded keeps the output pipe open is done when the
// command is, so its slot frees then and not when the child ends.
func TestGate_ABackgroundedChildDoesNotHoldTheSlot(t *testing.T) {
	t.Parallel()
	u := unitInRun(t, "")
	gateConfig(t, u.root, "(sleep 20 &) ; true", "")

	start := time.Now()
	gate := startTP(t, u.worktree, u.env, "gate")
	code, ok := gate.exited(t, 10*time.Second)
	require.True(t, ok, "tp gate returns when its command exits, not when the backgrounded sleep does")
	require.Equal(t, 0, code, gate.stderr.String())
	assert.Less(t, time.Since(start), 10*time.Second)

	next := startTP(t, u.worktree, u.env, "gate")
	code, ok = next.exited(t, 10*time.Second)
	require.True(t, ok, "the slot is free for the next gate: %s", next.stderr.String())
	assert.Equal(t, 0, code, next.stderr.String())
}

// TestGate_ATPCallInsideAGateTakesNoSlotOfTheRun: the run's slot variables
// stop at the gate, so a tp gate the gate's own command runs, in another
// project, is a call by hand and does not wait for the slot its caller holds.
func TestGate_ATPCallInsideAGateTakesNoSlotOfTheRun(t *testing.T) {
	t.Parallel()
	inner := projectGated(t, "true")
	outer := projectGated(t, "cd "+inner+" && "+binaryPath+" --json gate")
	runDir := engine.RunDir(outer, "01JTESTRUN0000000000000000")

	gate := startTP(t, outer, []string{engine.EnvRunDir + "=" + runDir, engine.EnvUnitSeq + "=1"}, "gate")
	code, ok := gate.exited(t, 20*time.Second)
	require.True(t, ok, "the inner tp gate does not wait for the outer gate's slot")
	assert.Equal(t, 0, code, gate.stderr.String())
}

// TestGate_ATimedOutGateFreesItsSlotAtTheTimeout: a gate killed at
// gate_timeout_seconds frees its slot then, even when the command's own child
// would run on and hold the pipe, so a gate waiting behind it can start.
func TestGate_ATimedOutGateFreesItsSlotAtTheTimeout(t *testing.T) {
	t.Parallel()
	u := unitInRun(t, "")
	gateConfig(t, u.root, "sleep 60; true", `,"gate_timeout_seconds":30`)

	start := time.Now()
	gate := startTP(t, u.worktree, u.env, "gate")
	code, ok := gate.exited(t, 45*time.Second)
	require.True(t, ok, "the timeout ends the gate, sleep included")
	assert.Equal(t, 4, code, gate.stderr.String())
	assert.Contains(t, gate.stderr.String(), "gate timed out after 30s")
	assert.Less(t, time.Since(start), 45*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	slot, err := engine.AcquireGateSlot(ctx, u.slots, 1, false)
	require.NoError(t, err, "the timed-out gate's slot is free")
	slot.Release()
}

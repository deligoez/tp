package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestGate_InAWorktreeGatesTheWorktreesCode: tp gate in a unit's worktree runs
// the gate on the worktree's checkout, as the unit's tp done would, and a red
// gate exits 4 like a red close.
func TestGate_InAWorktreeGatesTheWorktreesCode(t *testing.T) {
	t.Parallel()
	root, worktree, taskFile := pendingProject(t)
	env := append([]string{"TP_FILE=" + taskFile}, outsideRun...)

	stdout, stderr, code := runTPEnv(t, worktree, env, "gate")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout, `"passed": true`)
	assert.FileExists(t, filepath.Join(worktree, "gate-ran"), "the worktree's checkout is gated")
	assert.NoFileExists(t, filepath.Join(root, "gate-ran"), "the main tree is not")

	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"quality_gate":"echo worktree-gate-red; false"}}`), 0o600))
	_, stderr, code = runTPEnv(t, worktree, env, "gate")
	assert.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, "worktree-gate-red")
	status, _, _ := runTP(t, root, "show", "t1")
	assert.Contains(t, status, `"status": "wip"`, "tp gate closes nothing")
}

// TestGate_ThenDoneOnTheSameTreeRunsOneGate: inside a run, tp gate's pass on
// a clean checkout stands for the unit's following close of that same tree
// with that same gate command, so one full gate runs, not two; the close
// still records the gated tree for the main branch to reuse.
func TestGate_ThenDoneOnTheSameTreeRunsOneGate(t *testing.T) {
	t.Parallel()
	u := unitInRun(t, "")
	// The gate counts its runs in a file outside the repository, so it
	// leaves the checkout clean.
	count := filepath.Join(t.TempDir(), "count")
	require.NoError(t, os.WriteFile(filepath.Join(u.root, ".tp", "config.json"),
		[]byte(`{"workflow":{"quality_gate":"echo ran >> `+count+`","run_max_parallel_gates":1}}`), 0o600))

	_, stderr, code := runTPEnv(t, u.worktree, u.env, "gate")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTPEnv(t, u.worktree, u.env, "done", "t1", "implemented and verified", "--commit", u.sha)
	require.Equal(t, 0, code, stderr)

	runs, err := os.ReadFile(count) //nolint:gosec // the test's own temp file
	require.NoError(t, err)
	assert.Equal(t, "ran\n", string(runs), "one full gate for tp gate and the close together")
	assert.Contains(t, stderr, "gate reused from tp gate")
	pending, err := engine.ReadPendingCloses(u.root, u.taskFile)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.NotEmpty(t, pending[0].GatedTree, "the main branch can still reuse the pass")
}

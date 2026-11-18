package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

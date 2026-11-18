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

// TestGate_ThenDoneOnTheMainTreeRunsOneGate: a unit on the main tree (a run
// at run_max_parallel 1) closes its task itself, and that close stands on its
// tp gate pass for the same tree and gate command as a worktree close does.
// The claimed task file is the one difference from HEAD, and it is tp's own.
func TestGate_ThenDoneOnTheMainTreeRunsOneGate(t *testing.T) {
	t.Parallel()
	count := filepath.Join(t.TempDir(), "count")
	root := projectGated(t, "echo ran >> "+count)
	addTask(t, root, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "base")
	_, stderr, code := runTP(t, root, "claim", "t1")
	require.Equal(t, 0, code, stderr)
	env := []string{engine.EnvRunDir + "=" + engine.RunDir(root, "01JTESTRUN0000000000000000"), engine.EnvUnitSeq + "=1"}

	stdout, stderr, code := runTPEnv(t, root, env, "gate")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"recorded": true`, "the main tree's pass is recorded despite the claimed task file")
	_, stderr, code = runTPEnv(t, root, env, "done", "t1", "implemented and verified")
	require.Equal(t, 0, code, stderr)

	runs, err := os.ReadFile(count) //nolint:gosec // the test's own temp file
	require.NoError(t, err)
	assert.Equal(t, "ran\n", string(runs), "one full gate for tp gate and the close together")
	assert.Contains(t, stderr, "gate reused from tp gate")
}

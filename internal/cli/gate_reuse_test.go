package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// pickedPendingClose runs a unit's close in its worktree, which gates there
// and records the request, then picks the unit's commit onto the main branch
// as the driver does, and returns the picked sha.
func pickedPendingClose(t *testing.T) (root, taskFile, picked string) {
	t.Helper()
	root, worktree, taskFile := pendingProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "a.go"), []byte("package a\n"), 0o600))
	gitRun(t, worktree, "add", "a.go")
	gitRun(t, worktree, "commit", "-q", "-m", "add a")
	sha := gitRun(t, worktree, "rev-parse", "HEAD")
	_, stderr, code := runTPEnv(t, worktree, []string{"TP_FILE=" + taskFile}, "done", "t1", "implemented and verified", "--commit", sha)
	require.Equal(t, 0, code, stderr)

	pending, err := engine.ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.NotEmpty(t, pending[0].GatedTree, "a clean worktree records the tree its gate passed on")

	gitRun(t, root, "cherry-pick", sha)
	return root, taskFile, gitRun(t, root, "rev-parse", "HEAD")
}

// TestDone_ReusesTheUnitsGateOnTheSameTree: the main branch holds exactly the
// tree the unit's gate passed on, so the close records the unit's gate and
// does not run it again (the operator's decision of 2026-11-14).
func TestDone_ReusesTheUnitsGateOnTheSameTree(t *testing.T) {
	t.Parallel()
	root, taskFile, picked := pickedPendingClose(t)

	_, stderr, code := runTPEnv(t, root, []string{"TP_FILE=" + taskFile}, "done", "t1", "implemented and verified", "--commit", picked)
	require.Equal(t, 0, code, stderr)
	assert.NoFileExists(t, filepath.Join(root, "gate-ran"), "the gate is not run again on the same tree")
	assert.Contains(t, stderr, "gate reused from the unit's worktree")
	show, _, _ := runTP(t, root, "show", "t1")
	assert.Contains(t, show, `"gate_passed_at"`)
}

// TestDone_RunsTheGateWhenTheMainBranchHoldsMore: a file the unit never had
// sits in the main branch's working tree, so the tree the gate would run on is
// not the one the unit gated, and the gate runs.
func TestDone_RunsTheGateWhenTheMainBranchHoldsMore(t *testing.T) {
	t.Parallel()
	root, taskFile, picked := pickedPendingClose(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "kept.go"), []byte("package a\n"), 0o600))

	_, stderr, code := runTPEnv(t, root, []string{"TP_FILE=" + taskFile}, "done", "t1", "implemented and verified", "--commit", picked)
	require.Equal(t, 0, code, stderr)
	assert.FileExists(t, filepath.Join(root, "gate-ran"), "the gate runs on a tree the unit never gated")
	assert.NotContains(t, stderr, "gate reused")
}

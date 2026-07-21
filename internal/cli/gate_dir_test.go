package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eval resolves symlinks so macOS /private/var TempDir paths compare equal.
func eval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestGateDir_WalksUpToGitRootFromSubdir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	sub := filepath.Join(root, "spec")
	require.NoError(t, os.Mkdir(sub, 0o755))

	got := gateDir(filepath.Join(sub, "x.tasks.json"))
	assert.Equal(t, eval(t, root), eval(t, got),
		"gate must run from the repo root even when the task file is in a subdirectory")
}

func TestGateDir_GitAsFileWorktree(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /wt\n"), 0o600))

	got := gateDir(filepath.Join(root, "y.tasks.json"))
	assert.Equal(t, eval(t, root), eval(t, got), "a .git file (worktree) is a valid boundary")
}

func TestGateDir_NoGitReturnsEmptyOrRealBoundary(t *testing.T) {
	got := gateDir(filepath.Join(t.TempDir(), "z.tasks.json"))
	if got != "" {
		_, err := os.Stat(filepath.Join(got, ".git"))
		assert.NoError(t, err, "a non-empty gateDir must point at a real .git boundary")
	}
}

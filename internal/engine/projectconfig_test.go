package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func evalLink(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestDiscoverTPDir_FindsFromSubdir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	sub := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	got := DiscoverTPDir(sub)
	require.NotEmpty(t, got)
	assert.Equal(t, evalLink(t, filepath.Join(root, ".tp")), evalLink(t, got))
}

func TestDiscoverTPDir_TestsAnchorItself(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	got := DiscoverTPDir(root)
	assert.Equal(t, evalLink(t, filepath.Join(root, ".tp")), evalLink(t, got))
}

func TestDiscoverTPDir_HaltsAtGitBoundary(t *testing.T) {
	// .tp/ sits ABOVE the git boundary; the walk must stop at the boundary
	// and never read it.
	outer := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outer, ".tp"), 0o755))
	repo := filepath.Join(outer, "repo")
	require.NoError(t, os.Mkdir(repo, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(repo, ".git"), 0o755))

	assert.Empty(t, DiscoverTPDir(repo), "must not read a .tp/ above the git boundary")
}

func TestDiscoverTPDir_GitAsFileIsBoundary(t *testing.T) {
	outer := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outer, ".tp"), 0o755))
	repo := filepath.Join(outer, "wt")
	require.NoError(t, os.Mkdir(repo, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /x\n"), 0o600))

	assert.Empty(t, DiscoverTPDir(repo), "a .git file (worktree) is a boundary too")
}

func TestDiscoverTPDir_NoneReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	assert.Empty(t, DiscoverTPDir(root))
}

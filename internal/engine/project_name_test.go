package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectName_IsTheRepositoryRootsDirectory: from a subdirectory, the
// name is the repository root's, not the subdirectory's.
func TestProjectName_IsTheRepositoryRootsDirectory(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "acme-shop")
	sub := filepath.Join(root, "spec", "deep")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, exec.Command("git", "init", "-q", root).Run())
	assert.Equal(t, "acme-shop", ProjectName(sub))
}

// TestProjectName_OutsideARepositoryIsTheDirectorysOwn: no git, the
// directory itself names the project.
func TestProjectName_OutsideARepositoryIsTheDirectorysOwn(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "loose-notes")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	assert.Equal(t, "loose-notes", ProjectName(dir))
}

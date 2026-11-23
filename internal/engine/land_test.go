package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tp land refuses a task file of another project, so a project below the
// worktree root is entered first.
func TestLandCommandEntersAProjectBelowTheWorktreeRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "proj", ".tp"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".tp"), 0o750))

	assert.Equal(t, "(cd proj && tp land --file spec.tasks.json)", landCommand(home, "proj/spec.tasks.json"))
	assert.Equal(t, "tp land --file spec.tasks.json", landCommand(home, "spec.tasks.json"))
}

// A hint run from the repository root names a task file by its path from
// there, also for a project below it.
func TestRepoRelativePathIsFromTheWorktreeRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cmd := exec.Command("git", "init", "-q", home)
	require.NoError(t, cmd.Run())
	require.NoError(t, os.MkdirAll(filepath.Join(home, "proj", ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(home, "proj", "spec.tasks.json"), []byte("{}"), 0o600))

	assert.Equal(t, "proj/spec.tasks.json", RepoRelativePath(filepath.Join(home, "proj", "spec.tasks.json")))
}

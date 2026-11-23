package engine

import (
	"os"
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

	assert.Equal(t, "cd proj && tp land --file spec.tasks.json", landCommand(home, "proj/spec.tasks.json"))
	assert.Equal(t, "tp land --file spec.tasks.json", landCommand(home, "spec.tasks.json"))
}

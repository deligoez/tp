package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSetLocal_WritesDefault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	t.Chdir(root)

	require.NoError(t, runSetLocal([]string{"defaults.compact=true"}))

	lc, _, err := engine.LoadLocalConfig(filepath.Join(root, ".tp"))
	require.NoError(t, err)
	assert.True(t, lc.Defaults["compact"], "the flag default is written to local.json")

	// local.json is git-ignored via the .tp/.gitignore.
	data, err := os.ReadFile(filepath.Join(root, ".tp", ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "local.json")
}

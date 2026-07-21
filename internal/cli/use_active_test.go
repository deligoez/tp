package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetActiveFile_StoresProjectRelativeInLocalJSON(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "spec"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "spec", "x.tasks.json"), []byte("{}"), 0o600))
	t.Chdir(root)

	require.NoError(t, setActiveFile("spec/x.tasks.json"))

	lc, _, err := engine.LoadLocalConfig(filepath.Join(root, ".tp"))
	require.NoError(t, err)
	require.NotNil(t, lc.Active)
	assert.Equal(t, "spec/x.tasks.json", *lc.Active, "stored project-root-relative")

	_, statErr := os.Stat(filepath.Join(root, ".tp-active"))
	assert.True(t, os.IsNotExist(statErr), "tp use no longer writes a .tp-active marker")
}

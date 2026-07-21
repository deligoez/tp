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

func TestClearActiveFile_RemovesActiveKeepsDefaultsDropsLegacy(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"),
		[]byte(`{"active":"x.tasks.json","defaults":{"compact":true}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp-active"), []byte("old\n"), 0o600))
	t.Chdir(root)

	require.NoError(t, clearActiveFile())
	lc, _, err := engine.LoadLocalConfig(filepath.Join(root, ".tp"))
	require.NoError(t, err)
	assert.Nil(t, lc.Active, "active key removed")
	assert.True(t, lc.Defaults["compact"], "flag defaults are preserved")

	_, statErr := os.Stat(filepath.Join(root, ".tp-active"))
	assert.True(t, os.IsNotExist(statErr), "leftover legacy .tp-active is removed")

	require.NoError(t, clearActiveFile(), "clearing again is a no-op")
}

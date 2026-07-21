package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanProjectTaskFiles_CollectsRootAndSubdirs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.tasks.json"), []byte("{}"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "chapters", "one"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "chapters", "one", "b.tasks.json"), []byte("{}"), 0o600))
	// A non-task file is ignored.
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.md"), []byte("x"), 0o600))

	files, err := ScanProjectTaskFiles(root)
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Contains(t, files, filepath.Join(root, "a.tasks.json"))
	assert.Contains(t, files, filepath.Join(root, "chapters", "one", "b.tasks.json"))
}

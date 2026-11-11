package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// importedAtOf returns the task file's imported_at, nil when the key is absent.
func importedAtOf(t *testing.T, taskFilePath string) *time.Time {
	t.Helper()
	data, err := os.ReadFile(taskFilePath)
	require.NoError(t, err)
	var tf struct {
		ImportedAt *time.Time `json:"imported_at"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	return tf.ImportedAt
}

// TestImport_WritesImportedAtAndEveryImportMovesIt is spec/1.8.0.md §15 and
// §18 row 11's import half: tp import writes imported_at, in UTC, and a
// second import into the same file, --force included, rewrites it later.
func TestImport_WritesImportedAtAndEveryImportMovesIt(t *testing.T) {
	t.Parallel()
	dir, importPath, taskFilePath := importLockSetup(t)
	require.Nil(t, importedAtOf(t, taskFilePath), "an init shell carries no imported_at")

	before := time.Now().UTC()
	_, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s", stderr)
	first := importedAtOf(t, taskFilePath)
	require.NotNil(t, first)
	assert.Equal(t, time.UTC, first.Location())
	assert.False(t, first.Before(before), "imported_at is the import's own time")

	time.Sleep(2 * time.Millisecond)
	_, stderr, code = runTP(t, dir, "import", importPath, "--force")
	require.Equal(t, 0, code, "re-import failed: %s", stderr)
	second := importedAtOf(t, taskFilePath)
	require.NotNil(t, second)
	assert.True(t, second.After(*first), "a second import moves imported_at")
}

// TestImport_FileImportedBeforeTheReleaseGainsNoImportedAt is §15's old
// file: a task file written with no imported_at loads, and a write other than
// an import leaves it with none, so no time is reported that was never
// recorded.
func TestImport_FileImportedBeforeTheReleaseGainsNoImportedAt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Old Spec\n## 1. Setup\nSetup section.\n"), 0o600))
	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFilePath, []byte(importLockTaskFile), 0o600))

	_, stderr, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim failed: %s", stderr)
	assert.Nil(t, importedAtOf(t, taskFilePath))
}

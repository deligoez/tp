package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boundImportDir returns a project holding bindingChain, spec.md, a
// spec.tasks.json shell that tp init --epic a bound, and the path of an import
// document naming epic when it is not empty.
func boundImportDir(t *testing.T, epic string) (dir, importPath string) {
	t.Helper()
	dir = epicGraphDir(t, bindingChain)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Import Lock Spec\n## 1. Setup\nSetup section.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md", "--epic", "a")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	doc := importLockTaskFile
	if epic != "" {
		doc = strings.Replace(doc, `"spec": "spec.md",`, `"spec": "spec.md", "epic": "`+epic+`",`, 1)
	}
	importPath = filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))
	return dir, importPath
}

// TestImport_KeepsTheFileEpicBindingAndBoundAt is the reproduction: tp init
// --epic a, then tp import of a document carrying no epic key, left the file
// unbound with no bound_at. The import keeps both, bound_at unchanged, as it
// keeps the workflow block.
func TestImport_KeepsTheFileEpicBindingAndBoundAt(t *testing.T) {
	t.Parallel()
	dir, importPath := boundImportDir(t, "")
	taskFile := filepath.Join(dir, "spec.tasks.json")
	before := boundAtOf(t, taskFile)
	require.NotNil(t, before)

	_, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s", stderr)
	assert.Equal(t, "a", readBoundEpic(t, dir), "the import keeps the binding")
	after := boundAtOf(t, taskFile)
	require.NotNil(t, after, "the import keeps bound_at")
	assert.True(t, after.Equal(*before), "bound_at is unchanged")
}

// TestImport_ADocumentNamingTheSameEpicKeepsBoundAt: a document naming the
// file's own epic imports, and bound_at stays the binding's time.
func TestImport_ADocumentNamingTheSameEpicKeepsBoundAt(t *testing.T) {
	t.Parallel()
	dir, importPath := boundImportDir(t, "a")
	taskFile := filepath.Join(dir, "spec.tasks.json")
	before := boundAtOf(t, taskFile)
	require.NotNil(t, before)

	_, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s", stderr)
	assert.Equal(t, "a", readBoundEpic(t, dir))
	after := boundAtOf(t, taskFile)
	require.NotNil(t, after)
	assert.True(t, after.Equal(*before), "bound_at is unchanged")
}

// TestImport_ADocumentNamingAnotherEpicIsRefusedAndWritesNothing: a document
// naming b over a file bound to a exits 1 naming both epics and tp set
// --epic, and leaves the task file byte-identical.
func TestImport_ADocumentNamingAnotherEpicIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir, importPath := boundImportDir(t, "b")
	taskFile := filepath.Join(dir, "spec.tasks.json")
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, "import", importPath)
	assert.Equal(t, 1, code, "stderr: %s", stderr)
	var refusal struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(stderr), &refusal), "stderr: %s", stderr)
	assert.Contains(t, refusal.Error, `epic "a"`)
	assert.Contains(t, refusal.Error, `epic "b"`)
	assert.Contains(t, refusal.Hint, "tp set --epic")
	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "a refused import writes nothing")
}

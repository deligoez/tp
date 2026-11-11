package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportedBoundFiles runs tp epic export in dir and returns epic id's bound
// file entries as raw key maps, so a test can tell an absent key from a zero one.
func exportedBoundFiles(t *testing.T, dir, id string) []map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var p struct {
		Epics []struct {
			ID    string                       `json:"id"`
			Files []map[string]json.RawMessage `json:"files"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	for _, e := range p.Epics {
		if e.ID == id {
			return e.Files
		}
	}
	t.Fatalf("epic %s not in the export: %s", id, stdout)
	return nil
}

// rawTime decodes a JSON time value.
func rawTime(t *testing.T, raw json.RawMessage) time.Time {
	t.Helper()
	var at time.Time
	require.NoError(t, json.Unmarshal(raw, &at), "value: %s", raw)
	return at
}

// TestEpicExport_CarriesTheBoundFileTimes is spec/1.8.0.md §15: after tp set
// --epic a and tp import, tp epic export shows bound_at and imported_at under
// a's bound file, each the time the task file recorded. An export that drops
// either key turns it red.
func TestEpicExport_CarriesTheBoundFileTimes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(bindingChain), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Import Lock Spec\n## 1. Setup\nSetup section.\n"), 0o600))
	initGitRepo(t, dir)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	setEpic(t, dir, "a")
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(importLockTaskFile), 0o600))
	_, stderr, code = runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s", stderr)

	taskFile := filepath.Join(dir, "spec.tasks.json")
	boundAt, importedAt := boundAtOf(t, taskFile), importedAtOf(t, taskFile)
	require.NotNil(t, boundAt, "the import keeps the binding's time")
	require.NotNil(t, importedAt)

	files := exportedBoundFiles(t, dir, "a")
	require.Len(t, files, 1)
	require.Contains(t, files[0], "bound_at")
	require.Contains(t, files[0], "imported_at")
	assert.True(t, rawTime(t, files[0]["bound_at"]).Equal(*boundAt))
	assert.True(t, rawTime(t, files[0]["imported_at"]).Equal(*importedAt))
}

// TestEpicExport_ABoundFileWithoutTimesOmitsBoth is §15's file written before
// the release: a bound file carrying neither time exports with neither key,
// and its entry keeps exactly the keys it had. A null written for an absent
// time turns it red.
func TestEpicExport_ABoundFileWithoutTimesOmitsBoth(t *testing.T) {
	t.Parallel()
	dir := boundEpicDir(t, epicChain)

	files := exportedBoundFiles(t, dir, "b")
	require.Len(t, files, 1)
	keys := make([]string, 0, len(files[0]))
	for k := range files[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assert.Equal(t, []string{"audit", "ground", "path", "phase", "review", "tasks"}, keys)
	assert.JSONEq(t, `"b.tasks.json"`, string(files[0]["path"]))
	assert.JSONEq(t, `{"open": 0, "wip": 0, "done": 0}`, string(files[0]["tasks"]))
}

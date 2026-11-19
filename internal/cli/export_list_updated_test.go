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

// exportListUpdatedAt runs tp export --list in dir and returns each row's
// raw updated_at by path, absent rows' keys missing from the inner map.
func exportListUpdatedAt(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "export", "--list")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Files []map[string]json.RawMessage `json:"files"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	got := map[string]json.RawMessage{}
	for _, row := range out.Files {
		var path string
		require.NoError(t, json.Unmarshal(row["path"], &path))
		got[path] = row["updated_at"]
	}
	return got
}

// TestExportList_UpdatedAtIsTheLatestTimeTheFileRecords: a row's updated_at
// is the latest of its tasks' started_at and closed_at and the file's
// bound_at and imported_at, read from the file and never from its mtime,
// which a checkout resets; with none of them, the latest recorded review or
// audit round; with neither, the key is absent.
func TestExportList_UpdatedAtIsTheLatestTimeTheFileRecords(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other spec\n\n## 1. A\n\na.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "other.md")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "timed.md"), []byte("# Timed\n\n## 1. A\n\na.\n"), 0o600))
	_, stderr, code = runTP(t, dir, "init", "timed.md")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "--file", "timed.tasks.json", "add", `{"id":"t1","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a done","source_sections":["## 1. A"]}`)
	require.Equal(t, 0, code, stderr)
	timedPath := filepath.Join(dir, "timed.tasks.json")
	raw, err := os.ReadFile(timedPath)
	require.NoError(t, err)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(raw, &tf))
	tf["bound_at"], tf["imported_at"] = "2026-11-01T10:00:00Z", "2026-11-02T10:00:00Z"
	task, ok := tf["tasks"].([]any)[0].(map[string]any)
	require.True(t, ok)
	task["started_at"], task["closed_at"] = "2026-11-04T10:00:00Z", "2026-11-03T10:00:00Z"
	raw, err = json.MarshalIndent(tf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(timedPath, raw, 0o600))
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(timedPath, old, old))

	got := exportListUpdatedAt(t, dir)
	require.Contains(t, got, "timed.tasks.json")
	assert.JSONEq(t, `"2026-11-04T10:00:00Z"`, string(got["timed.tasks.json"]))

	rounds := exportReviewRounds(t, exportJSON(t, dir, "--file", "spec.tasks.json"))
	last, ok := rounds[len(rounds)-1].(map[string]any)
	require.True(t, ok)
	recorded, err := time.Parse(time.RFC3339, last["recorded_at"].(string))
	require.NoError(t, err)
	require.NotNil(t, got["spec.tasks.json"], "a file with rounds and no task times dates from its latest round")
	var specAt time.Time
	require.NoError(t, json.Unmarshal(got["spec.tasks.json"], &specAt))
	assert.True(t, recorded.Equal(specAt), "updated_at %s, latest round %s", specAt, recorded)

	assert.Nil(t, got["other.tasks.json"], "a file that records no time carries no updated_at")
}

package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicFileTask is one open task, the shape every --epic fixture file holds.
const epicFileTask = `{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}`

// epicFileRepo is a git repository whose graph holds the root epics b and
// u, with x.tasks.json bound to no epic, so plain discovery is ambiguous.
func epicFileRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(`{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "b", "title": "B", "milestone": "A", "root": true},
    {"id": "u", "title": "U", "milestone": "A", "root": true}
  ]}`), 0o600))
	writeEpicFile(t, dir, "x", "", "open")
	return dir
}

// writeEpicFile writes <base>.md and <base>.tasks.json bound to epic (none
// when ""), holding one task of status.
func writeEpicFile(t *testing.T, dir, base, epic, status string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"), []byte("# "+base+"\n"), 0o600))
	task := epicFileTask
	if status != "open" {
		task = fmt.Sprintf(`{"id":"t1","title":"T","status":%q,"depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}`, status)
	}
	binding := ""
	if epic != "" {
		binding = fmt.Sprintf(`"epic":%q,`, epic)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".tasks.json"),
		[]byte(`{"version":1,"spec":"`+base+`.md",`+binding+`"tasks":[`+task+`]}`), 0o600))
}

// TestEpicFlag_TheOnlyBoundFileIsUsed is F8: b binds b1.tasks.json alone, so
// tp ready --epic b reads it although two task files make discovery ambiguous.
func TestEpicFlag_TheOnlyBoundFileIsUsed(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	writeEpicFile(t, dir, "b1", "b", "open")
	initGitRepo(t, dir)

	stdout, stderr, code := runTP(t, dir, "ready", "--epic", "b", "--ids")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, "t1")
}

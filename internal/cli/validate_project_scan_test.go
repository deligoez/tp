package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeProjectConfigDir seeds a project whose .tp/config.json carries workflow.
func writeProjectConfigDir(t *testing.T, workflow string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
		[]byte(`{"workflow":`+workflow+`}`), 0o600))
	return dir
}

// End to end: the commit_strategy comparison is reachable through the command,
// not only through workflowDeviations.
func TestValidateProject_ReportsCommitStrategyDeviation(t *testing.T) {
	dir := writeProjectConfigDir(t, `{"commit_strategy":"hc"}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"commit_strategy":"builtin"}}`), 0o600))

	stdout, stderr, code := runTP(t, dir, "validate", "--project")
	require.Equal(t, 0, code, "validate --project: %s", stderr)

	var payload struct {
		Deviations []map[string]any `json:"deviations"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	fields := make([]string, 0, len(payload.Deviations))
	for _, d := range payload.Deviations {
		fields = append(fields, d["field"].(string))
	}
	assert.Contains(t, fields, "commit_strategy",
		"a task file contradicting the project commit_strategy is reported")
}

// A directory the scan cannot read aborts the walk, so every task file after it
// goes unexamined. The report must say so: an incomplete scan that prints an
// empty deviation list is indistinguishable from a genuinely clean project.
func TestValidateProject_ReportsAnIncompleteScan(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a 0000 directory, so the scan cannot be made to fail")
	}
	dir := writeProjectConfigDir(t, `{"review_max_rounds":8}`)

	// a-locked sorts before z.tasks.json, so the walk aborts there and never
	// reaches the task file that genuinely deviates.
	locked := filepath.Join(dir, "a-locked")
	require.NoError(t, os.Mkdir(locked, 0o755))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":3}}`), 0o600))

	stdout, stderr, code := runTP(t, dir, "validate", "--project")
	require.Equal(t, 0, code, "an unreadable subtree is reported, not fatal: %s", stderr)

	var payload struct {
		Skipped []string `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.NotEmpty(t, payload.Skipped,
		"an incomplete scan must not report as a clean one")
	assert.Contains(t, payload.Skipped[0], "a-locked",
		"the unreadable location is named in skipped")
	assert.Contains(t, stderr, "a-locked",
		"the unreadable location is warned about on stderr")
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicImportPayload writes body as an import payload outside the .tp/
// directory and returns its path.
func epicImportPayload(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "graph.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// TestEpicImport_ChainNeedsForceToBeReplaced is §9 row 16: an import that
// always replaced would overwrite the chain without --force.
func TestEpicImport_ChainNeedsForceToBeReplaced(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := epicImportPayload(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "solo", "title": "Solo", "milestone": "A"}
  ]
}
`)

	_, stderr, code := runTP(t, dir, "epic", "import", payload)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "--force")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused import writes nothing")

	_, stderr, code = runTP(t, dir, "epic", "import", payload, "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var g struct {
		Epics []struct {
			ID string `json:"id"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	require.Len(t, g.Epics, 1)
	assert.Equal(t, "solo", g.Epics[0].ID, "--force replaces the file with the payload")
}

// TestEpicImport_PayloadWithACycleIsWrittenAndReported is D4: import does not
// refuse a payload's lint errors; it writes the payload and reports them.
func TestEpicImport_PayloadWithACycleIsWrittenAndReported(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, "")
	payload := epicImportPayload(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "x", "title": "X", "milestone": "A", "depends_on": [{"id": "y", "why": "x reads y"}]},
    {"id": "y", "title": "Y", "milestone": "A", "depends_on": [{"id": "x", "why": "y reads x"}]}
  ]
}
`)

	stdout, stderr, code := runTP(t, dir, "epic", "import", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		File     string   `json:"file"`
		Epics    []string `json:"epics"`
		Findings []struct {
			Rule  string   `json:"rule"`
			Epics []string `json:"epics"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, []string{"x", "y"}, out.Epics)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "cycle", out.Findings[0].Rule)
	assert.Equal(t, []string{"x", "y"}, out.Findings[0].Epics)

	lint, _, code := runEpicLint(t, dir)
	require.Equal(t, 1, code)
	assert.Equal(t, 1, lint.Errors, "the cycle was written to the file")
}

// TestEpicImport_TrailingDataAfterTheGraphIsAUsageError: a decoder reading
// only the first JSON value would import the graph and ignore the junk after it.
func TestEpicImport_TrailingDataAfterTheGraphIsAUsageError(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, "")
	payload := epicImportPayload(t, `{"version": 1, "milestones": [["A"]], "epics": []} junk`)

	_, stderr, code := runTP(t, dir, "epic", "import", payload)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "invalid epic graph JSON")
	_, err := os.Stat(filepath.Join(dir, ".tp", "epics.json"))
	assert.True(t, os.IsNotExist(err), "a refused import writes nothing")
}

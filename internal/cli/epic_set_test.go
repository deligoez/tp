package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicSet_MilestonesWithNoFileCreatesIt is the first half of §9 row 20: a
// set that refused an absent file would leave no graph file behind.
func TestEpicSet_MilestonesWithNoFileCreatesIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, "")
	_, err := os.Stat(filepath.Join(dir, ".tp", "epics.json"))
	require.True(t, os.IsNotExist(err), "the fixture holds no graph file")

	stdout, stderr, code := runTP(t, dir, "epic", "set", `milestones=[["A"]]`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		File string `json:"file"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, "epics.json", filepath.Base(out.File))

	var g struct {
		Milestones [][]string `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	assert.Equal(t, [][]string{{"A"}}, g.Milestones)
}

// TestEpicSet_FieldAddingAnErrorIsRefusedAndWritesNothing: a set moving an epic
// to a milestone no track names adds unknown-milestone, so it exits 1 and the
// file stays byte-identical.
func TestEpicSet_FieldAddingAnErrorIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"}
  ]
}
`)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "set", "a", "milestone=Z")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "unknown-milestone")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused set writes nothing")
}


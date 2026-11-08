package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readEpicGraphFile returns dir's .tp/epics.json bytes.
func readEpicGraphFile(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)
	return data
}

// TestEpicAdd_EdgeWithoutWhyIsRefusedAndWritesNothing is §9 row 1: a writer
// that validated after writing would leave b in the file.
func TestEpicAdd_EdgeWithoutWhyIsRefusedAndWritesNothing(t *testing.T) {
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

	_, stderr, code := runTP(t, dir, "epic", "add",
		`{"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "a"}]}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "missing-why")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused add writes nothing")
}

// TestEpicAdd_PreExistingErrorDoesNotBlockButANewOneDoes is §9 row 2: a
// writer refusing any after-state error would refuse the first add, and one
// comparing by rule alone would accept the second.
func TestEpicAdd_PreExistingErrorDoesNotBlockButANewOneDoes(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "z", "why": "a reads z"}]},
    {"id": "b", "title": "B", "milestone": "A"}
  ]
}
`)
	stdout, stderr, code := runTP(t, dir, "epic", "add",
		`{"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "b", "why": "c reads b"}]}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		File string `json:"file"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, "epics.json", filepath.Base(out.File))
	afterFirst := readEpicGraphFile(t, dir)
	assert.Contains(t, string(afterFirst), `"c reads b"`)

	_, stderr, code = runTP(t, dir, "epic", "add",
		`{"id": "e", "title": "E", "milestone": "A", "depends_on": [{"id": "y", "why": "e reads y"}]}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "dangling-reference")
	assert.Contains(t, stderr, "e, y")
	assert.Equal(t, afterFirst, readEpicGraphFile(t, dir), "a refused add writes nothing")
}


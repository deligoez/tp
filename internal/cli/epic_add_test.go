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


package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicAdd_ARefusedBulkNamesThePayloadLine: an eleven-row payload whose
// fourth line has an edge to the first with an empty why is refused, and the
// error names the 1-based payload lines of both epics it involves, so a large
// payload need not be searched by id. A count from 0 would name lines 0, 3.
func TestEpicAdd_ARefusedBulkNamesThePayloadLine(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": []
}
`)
	var rows []string
	for i := 1; i <= 11; i++ {
		id := "e" + string(rune('a'+i-1))
		row := `{"id": "` + id + `", "title": "T ` + id + `", "milestone": "A"}`
		if i == 4 {
			row = `{"id": "` + id + `", "title": "T ` + id + `", "milestone": "A", "depends_on": [{"id": "ea", "why": " "}]}`
		}
		rows = append(rows, row)
	}
	bulk := filepath.Join(dir, "rows.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(strings.Join(rows, "\n")+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "epic", "add", "--bulk", bulk)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "missing-why")
	assert.Contains(t, stderr, "--bulk lines 1, 4", "the error names ea (line 1) and ed (line 4)")
}

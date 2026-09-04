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

// initTwoTaskProject creates an initialized project holding t1 and t2, the two
// rows the bulk-set NDJSON below addresses.
func initTwoTaskProject(t *testing.T) string {
	t.Helper()
	dir := initEntryProject(t)
	for _, taskJSON := range []string{
		`{"id":"t1","title":"Setup","estimate_minutes":5,"acceptance":"setup done","source_sections":["## 1. Setup"],"depends_on":[]}`,
		`{"id":"t2","title":"Models","estimate_minutes":5,"acceptance":"models done","source_sections":["## 2. Models"],"depends_on":[]}`,
	} {
		_, stderr, code := runTP(t, dir, "add", taskJSON)
		require.Equal(t, 0, code, "add failed: %s", stderr)
	}
	return dir
}

// TestSet_BulkOverLongLineAborts REVERSES the warn-and-continue contract this
// reader used to keep: it applied the rows read before the over-long line,
// dropped that row and every row after it, wrote the task file anyway, and
// exited 0 reporting "1 updated, 0 failed". A partial update reported as a
// complete one IS the silent data loss the warning announced — and the warning
// went to bare stderr, so a JSON-mode caller reading the exit code saw nothing.
// tp set --bulk now reads at the shared ndjsonLineCap and, past it, aborts
// exit 3 naming the file and the line, applying no row.
func TestSet_BulkOverLongLineAborts(t *testing.T) {
	t.Parallel()
	dir := initTwoTaskProject(t)
	before := readRawTaskFile(t, dir)

	ndjson := `{"id":"t1","field":"title","value":"Renamed first"}` + "\n" +
		strings.Repeat("x", 2*1024*1024) + "\n" +
		`{"id":"t2","field":"title","value":"Renamed second"}` + "\n"
	bulkPath := filepath.Join(dir, "sets.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "set", "--bulk", bulkPath)
	require.Equal(t, 3, code, "an over-long line is a file error, not a partial update: %s", stderr)

	e := errJSON(t, stderr)
	assert.Contains(t, e["error"], bulkPath, "the error names the file tp could not read")
	assert.Contains(t, e["error"], "line 2", "the error names the line that exceeded the cap")
	hint, ok := e["hint"].(string)
	require.True(t, ok, "an over-long line must carry a hint")
	assert.Contains(t, hint, "1MB", "the hint names the cap that was exceeded")

	assert.Equal(t, before, readRawTaskFile(t, dir),
		"the abort writes nothing: not even the rows read before the over-long line")
}

// TestSet_BulkReadsRowUpToTheCap pins the other side of the same move. 200KB is
// far past bufio's 64KB default this reader used to keep — that row was dropped
// with a warning — and well under the shared 1MB cap, so it is now an ordinary
// row. Without this the abort above would still pass at any cap, including one
// lower than the old default.
func TestSet_BulkReadsRowUpToTheCap(t *testing.T) {
	t.Parallel()
	dir := initTwoTaskProject(t)

	const bigLen = 200 * 1024
	row := `{"id":"t1","field":"acceptance","value":"` + strings.Repeat("x", bigLen) + `"}`
	bulkPath := filepath.Join(dir, "sets.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(row+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "set", "--bulk", bulkPath)
	require.Equal(t, 0, code, "a 200KB row is under the 1MB cap: %s", stderr)

	var tf struct {
		Tasks []struct {
			ID         string `json:"id"`
			Acceptance string `json:"acceptance"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(readRawTaskFile(t, dir), &tf))
	require.Len(t, tf.Tasks, 2)
	assert.Equal(t, bigLen, len(tf.Tasks[0].Acceptance), "the row is applied, not dropped")
}

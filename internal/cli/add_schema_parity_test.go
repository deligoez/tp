package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdd_BulkRefusesARowValidateWouldReject: a row without estimate_minutes
// used to be written at exit 0 and then fail the next tp validate. The bulk
// add applies every row or none, so the refusal writes nothing.
func TestAdd_BulkRefusesARowValidateWouldReject(t *testing.T) {
	t.Parallel()
	dir := initEntryProject(t)
	before := readRawTaskFile(t, dir)

	ndjson := validEntryTask + "\n" +
		`{"id":"t2","title":"Second","acceptance":"second","source_sections":["## 2. Models"]}` + "\n"
	bulkPath := filepath.Join(dir, "bulk.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "add", "--bulk", bulkPath)
	require.Equal(t, 1, code, "a row tp validate rejects must be refused: %s", stderr)
	e := errJSON(t, stderr)
	assert.Contains(t, e["error"], "estimate_minutes")
	assert.Equal(t, string(before), string(readRawTaskFile(t, dir)), "nothing may be written")
}

// TestSet_RefusesAValueValidateWouldReject: tp set takes the same line, singly
// and in bulk, and leaves the task as it was.
func TestSet_RefusesAValueValidateWouldReject(t *testing.T) {
	t.Parallel()
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add", validEntryTask)
	require.Equal(t, 0, code, stderr)
	before := readRawTaskFile(t, dir)

	_, stderr, code = runTP(t, dir, "set", "t1", "estimate_minutes=0")
	require.Equal(t, 1, code, "estimate_minutes=0 must be refused: %s", stderr)
	assert.Contains(t, errJSON(t, stderr)["error"], "estimate_minutes")
	assert.Equal(t, string(before), string(readRawTaskFile(t, dir)), "the single set wrote")

	bulkPath := filepath.Join(dir, "set.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(`{"id":"t1","field":"estimate_minutes","value":0}`+"\n"), 0o600))
	out, stderr, _ := runTP(t, dir, "set", "--bulk", bulkPath)
	assert.Contains(t, out+stderr, "estimate_minutes is required")
	assert.Contains(t, string(readRawTaskFile(t, dir)), `"estimate_minutes": 5`, "the bulk set wrote")
}

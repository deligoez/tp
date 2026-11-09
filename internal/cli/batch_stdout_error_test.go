package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envelopeCode decodes the code field of the error envelope on stderr.
func envelopeCode(t *testing.T, stderr string) int {
	t.Helper()
	var e struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal([]byte(stderr), &e), "stderr: %s", stderr)
	return e.Code
}

// TestClaimBatch_AResultItCannotWriteExitsThree: tp claim of several ids
// whose stdout cannot be written is a file error, exit 3, as in tp ground
// (TestOneStdoutWriteErrorIsOneClassification). A batch that only reported the
// write error exits 0 having claimed the tasks and printed nothing, and fails.
func TestClaimBatch_AResultItCannotWriteExitsThree(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "", "open", "open")

	stderr, code := runTPUnwritableStdout(t, dir, "claim", "t1", "t2")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Equal(t, 3, envelopeCode(t, stderr), "stderr: %s", stderr)
}

// TestDoneMulti_AResultItCannotWriteExitsThree: tp done of several ids whose
// stdout cannot be written exits 3, as tp claim's batch does. A tp done that
// only reported the write error exits 0 having printed nothing, and fails.
func TestDoneMulti_AResultItCannotWriteExitsThree(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "", "open", "open")

	stderr, code := runTPUnwritableStdout(t, dir, "done", "t1", "t2", "tested by the fixture")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Equal(t, 3, envelopeCode(t, stderr), "stderr: %s", stderr)
}

// TestDoneBatch_AResultItCannotWriteExitsThree: tp done --batch whose stdout
// cannot be written exits 3, as tp done of several ids does. A batch that only
// reported the write error exits 0 having printed nothing, and fails.
func TestDoneBatch_AResultItCannotWriteExitsThree(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "", "open", "open")
	batch := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(batch, []byte(`{"id":"t1","reason":"tested by the fixture"}`+"\n"), 0o600))

	stderr, code := runTPUnwritableStdout(t, dir, "done", "--batch", batch)
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Equal(t, 3, envelopeCode(t, stderr), "stderr: %s", stderr)
}

// TestClaimSingle_AResultItCannotWriteExitsThree: tp claim of one id whose
// stdout cannot be written exits 3, as tp claim of several ids does. A claim
// that only returned the write error exits 1 having printed nothing, and fails.
func TestClaimSingle_AResultItCannotWriteExitsThree(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "", "open")

	stderr, code := runTPUnwritableStdout(t, dir, "claim", "t1")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Equal(t, 3, envelopeCode(t, stderr), "stderr: %s", stderr)
}

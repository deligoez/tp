package cli_test

import (
	"encoding/json"
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

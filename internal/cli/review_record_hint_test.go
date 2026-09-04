// The audit-side mirror in audit_record_hint_test.go is deliberate: the same defect on both
// sides of the loop, answered by the same shared const, asserted separately so either side can
// regress alone. t.Parallel() tipped the pair over dupl's 150-token clone threshold.
//
//nolint:dupl // deliberate audit/review mirror, see above
package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewRecordMissingFindingsHint: --record is the flag every review round
// ends on, so a mistyped findings path is the loop's most-hit file error. The
// site passed no hint and so inherited the code-3 default — task-file advice
// ("run 'tp use <file>' … 'tp init <spec>'"), the wrong object entirely for the
// NDJSON the reviewers wrote. tp audit's --record was fixed first; this is the
// same defect on the review side, answered by the same shared const.
func TestReviewRecordMissingFindingsHint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	missing := filepath.Join(dir, "findigs.ndjson")
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", missing)
	require.Equal(t, 3, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Contains(t, payload["error"], missing, "the error names the path the caller typed")

	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "--record", "the hint names the flag whose path is wrong")
	assert.Contains(t, hint, "NDJSON", "the hint names the findings file")
	assert.NotContains(t, hint, "tp use", "task-file advice is the wrong object for a --record typo")
	assert.NotContains(t, hint, "tp init", "task-file advice is the wrong object for a --record typo")
}

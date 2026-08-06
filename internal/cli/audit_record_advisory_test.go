package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditRecordRolelessRowsAdviseOnce: the role-less-row advisory fired once
// per ROW on raw os.Stderr. Two costs, both real in the loop: it ignored
// --quiet, and an N-row results file paid N near-identical lines (48 rows cost
// 5367 bytes of unsuppressible stderr). One condition costs one Notice line
// carrying the count.
func TestAuditRecordRolelessRowsAdviseOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	rows := make([]string, 0, 48)
	for i := 0; i < 48; i++ {
		rows = append(rows, `{"id":"r","status":"FAIL","item_id":"i"}`)
	}
	ndjson := strings.Join(rows, "\n") + "\n"

	resultsPath := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(resultsPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--record", resultsPath)
	require.Equal(t, 0, code, "role-less rows are an advisory, not an error: %s", stderr)
	assert.Equal(t, 1, strings.Count(stderr, "missing the role field"),
		"48 role-less rows are one condition, so they cost one advisory: %q", stderr)
	assert.Contains(t, stderr, "48 row(s)", "the advisory carries the count the per-row form never gave")
	assert.Contains(t, stderr, resultsPath, "the advisory names the results file")

	// --quiet suppresses it: the advisory travels output.Notice, not raw stderr.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "results2.ndjson"), []byte(ndjson), 0o600))
	_, quietStderr, code := runTP(t, dir, "audit", "spec.md", "--record",
		filepath.Join(dir, "results2.ndjson"), "--quiet")
	require.Equal(t, 0, code)
	assert.Empty(t, quietStderr, "a quiet round pays no stderr for role-less rows: %q", quietStderr)
}

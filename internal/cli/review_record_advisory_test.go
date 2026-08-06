package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewRecordRolelessRowsAdviseOnce: the role-less-row advisory fired once
// per ROW on raw os.Stderr. Two costs, both real in the loop: it ignored
// --quiet, and an N-row findings file paid N near-identical lines. One
// condition costs one Notice line carrying the count. tp audit's --record was
// fixed first; review carried the same defect verbatim, and both now share one
// rolelessRows tracker so the wording cannot drift apart per phase.
func TestReviewRecordRolelessRowsAdviseOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	rows := make([]string, 0, 48)
	for i := 0; i < 48; i++ {
		rows = append(rows, `{"severity":"low","category":"consistency","location":"L1","finding":"f","suggestion":"s"}`)
	}
	ndjson := strings.Join(rows, "\n") + "\n"

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", findingsPath)
	require.Equal(t, 0, code, "role-less rows are an advisory, not an error: %s", stderr)
	assert.Equal(t, 1, strings.Count(stderr, "missing the role field"),
		"48 role-less rows are one condition, so they cost one advisory: %q", stderr)
	assert.Contains(t, stderr, "48 row(s)", "the advisory carries the count the per-row form never gave")
	assert.Contains(t, stderr, findingsPath, "the advisory names the findings file")

	// --quiet suppresses it: the advisory travels output.Notice, not raw stderr.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "findings2.ndjson"), []byte(ndjson), 0o600))
	_, quietStderr, code := runTP(t, dir, "review", "spec.md", "--record",
		filepath.Join(dir, "findings2.ndjson"), "--quiet")
	require.Equal(t, 0, code)
	assert.Empty(t, quietStderr, "a quiet round pays no stderr for role-less rows: %q", quietStderr)
}

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertRolelessRowsAdviseOnce drives one phase's --record over 48 role-less
// rows and asserts the advisory is emitted once, carries the count, names the
// file it read, and is suppressed by --quiet. tp audit's --record was fixed
// first and tp review carried the same defect verbatim; both now share one
// rolelessRows tracker, so both phases are checked through one body here rather
// than through two files that drift apart.
func assertRolelessRowsAdviseOnce(t *testing.T, phase, row, fileStem string) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	rows := make([]string, 0, 48)
	for i := 0; i < 48; i++ {
		rows = append(rows, row)
	}
	ndjson := strings.Join(rows, "\n") + "\n"

	path := filepath.Join(dir, fileStem+".ndjson")
	require.NoError(t, os.WriteFile(path, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, phase, "spec.md", "--record", path)
	require.Equal(t, 0, code, "role-less rows are an advisory, not an error: %s", stderr)
	assert.Equal(t, 1, strings.Count(stderr, "missing the role field"),
		"48 role-less rows are one condition, so they cost one advisory: %q", stderr)
	assert.Contains(t, stderr, "48 row(s)", "the advisory carries the count the per-row form never gave")
	assert.Contains(t, stderr, path, "the advisory names the file it read")

	// --quiet suppresses it: the advisory travels output.Notice, not raw stderr.
	second := filepath.Join(dir, fileStem+"2.ndjson")
	require.NoError(t, os.WriteFile(second, []byte(ndjson), 0o600))
	_, quietStderr, code := runTP(t, dir, phase, "spec.md", "--record", second, "--quiet")
	require.Equal(t, 0, code)
	assert.Empty(t, quietStderr, "a quiet round pays no stderr for role-less rows: %q", quietStderr)
}

// TestAuditRecordRolelessRowsAdviseOnce: the role-less-row advisory fired once
// per ROW on raw os.Stderr. Two costs, both real in the loop: it ignored
// --quiet, and an N-row results file paid N near-identical lines (48 rows cost
// 5367 bytes of unsuppressible stderr). One condition costs one Notice line
// carrying the count.
func TestAuditRecordRolelessRowsAdviseOnce(t *testing.T) {
	assertRolelessRowsAdviseOnce(t, "audit", `{"id":"r","status":"FAIL","item_id":"i"}`, "results")
}

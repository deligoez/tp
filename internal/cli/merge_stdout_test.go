package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ndjsonLines splits a merge's NDJSON stream into its parsed rows.
func ndjsonLines(t *testing.T, stream string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimRight(stream, "\n"), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row), "every stdout line is one NDJSON row: %q", line)
		rows = append(rows, row)
	}
	return rows
}

// TestReviewMerge_DashOutputSplitsRowsAndSummary pins v1.7.0 §2 for the review
// merge: `-o -` writes the merged NDJSON to stdout and the JSON summary to
// stderr, so the stream reads as rows alone. Without the dash branch `-o -`
// writes a file named "-" and prints the summary on stdout.
func TestReviewMerge_DashOutputSplitsRowsAndSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{goodFinding1, goodFinding2})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1, f2, "-o", "-")
	require.Equal(t, 0, code, "a clean merge exits 0: %s", stderr)

	rows := ndjsonLines(t, stdout)
	assert.Len(t, rows, 3, "stdout carries the merged rows and nothing else")

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &summary), "stderr carries the JSON summary: %s", stderr)
	assert.Equal(t, float64(3), summary["merged_count"])
	assert.Equal(t, "stdout", summary["output_path"], "the summary names where the rows went")
	assert.NoFileExists(t, dir+"/-", "`-o -` is not a file named -")
}

// TestAuditMerge_DashOutputSplitsRowsAndSummary is §2 for the audit merge,
// which shares the rule.
func TestAuditMerge_DashOutputSplitsRowsAndSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f1 := writeFindingsFile(t, dir, "a1.ndjson", []string{goodAuditRow1, goodAuditRow2})
	f2 := writeFindingsFile(t, dir, "a2.ndjson", []string{goodAuditRow3})

	stdout, stderr, code := runTPMerge(t, dir, "audit", "--merge", f1, f2, "-o", "-")
	require.Equal(t, 0, code, "a clean merge exits 0: %s", stderr)

	rows := ndjsonLines(t, stdout)
	assert.Len(t, rows, 3, "stdout carries the merged rows and nothing else")

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &summary), "stderr carries the JSON summary: %s", stderr)
	assert.Equal(t, "stdout", summary["output_path"], "the summary names where the rows went")
	assert.NoFileExists(t, dir+"/-", "`-o -` is not a file named -")
}


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

// TestMerge_RefusedDashOutputWritesNothingToStdout pins §2's refusal half: an
// empty stream is how a converged round reports itself, so a merge that exits
// non-zero under `-o -` writes no byte to stdout, in either merge. The summary
// still reaches stderr, ahead of the failure envelope on stderr's last line.
func TestMerge_RefusedDashOutputWritesNothingToStdout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		command string
		dropped string
		legal   string
	}{
		{"review", `{"role":"implementer","severity":"high","finding":"missing bound","evidence":"read the cited section"}`, goodFinding2},
		{"audit", `{"role":"go-safety","item_id":"a"}`, goodAuditRow2},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			dropped := writeFindingsFile(t, dir, "dropped.ndjson", []string{tc.dropped})
			legal := writeFindingsFile(t, dir, "legal.ndjson", []string{tc.legal})

			stdout, stderr, code := runTPMerge(t, dir, tc.command, "--merge", "--json", dropped, legal, "-o", "-")
			require.Equal(t, 1, code, "the dropped input refuses the merge: %s", stderr)
			assert.Empty(t, stdout, "a refused merge writes nothing to stdout")

			lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
			var envelope map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &envelope), "the envelope is stderr's last line: %s", stderr)
			assert.Equal(t, float64(1), envelope["code"])
			assert.Contains(t, stderr, `"merged_count"`, "the summary still reaches stderr")
			assert.NotContains(t, stderr, `"output_path"`, "no rows were written, so the summary names no output path")
		})
	}
}

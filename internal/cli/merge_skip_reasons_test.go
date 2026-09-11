package cli_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergeRefusal parses the JSON envelope a refused merge ends stderr with and
// returns its error and hint.
func mergeRefusal(t *testing.T, stderr string) (msg, hint string) {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload),
		"the refusal must be a JSON envelope: %s", stderr)
	return payload.Error, payload.Hint
}

// formatBlame is the part of the format hint that blames the file's JSON.
const formatBlame = "trailing comma"

// TestReviewMerge_MissingFieldRefusalNamesTheFieldAndTheLines is the BUGS.md
// repro: three rows that are valid JSON and lack only `evidence`. The refusal
// used to blame the file's JSON format ("a trailing comma or a wrapping
// array"), which is not what is wrong with any of the three, and named no
// line. Every warning now names its line, the error names the field and the
// lines, and the hint says what a row needs instead of blaming the format.
func TestReviewMerge_MissingFieldRefusalNamesTheFieldAndTheLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lines := make([]string, 0, 3)
	for n := 1; n <= 3; n++ {
		lines = append(lines, fmt.Sprintf(
			`{"role":"implementer","severity":"high","class":"gap","location":"§%d","finding":"f%d"}`, n, n))
	}
	rev := writeFindingsFile(t, dir, "rev.ndjson", lines)

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", rev)
	require.Equal(t, 1, code, "an input whose every row lacks evidence parses nothing: %s", stderr)

	for n := 1; n <= 3; n++ {
		assert.Contains(t, stderr, fmt.Sprintf("warning: skipping incomplete line (missing evidence) in %s:%d\n", rev, n),
			"each skip warning names its line")
	}
	msg, hint := mergeRefusal(t, stderr)
	assert.Contains(t, msg, rev+" (missing evidence: lines 1-3)", "the error names the field and the lines")
	assert.NotContains(t, msg, "invalid JSON", "no line was invalid JSON")
	assert.NotContains(t, hint, formatBlame, "the JSON was fine, so the hint must not blame its format")
	assert.Contains(t, hint, "severity, finding, location and evidence", "the hint names what every row needs")
}

// TestAuditMerge_MissingFieldRefusalNamesTheFieldAndTheLines is the audit half
// of the same defect: rows lacking `status`. Line numbers are the file's own,
// blank lines included, so the operator can go straight to them.
func TestAuditMerge_MissingFieldRefusalNamesTheFieldAndTheLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rows := writeFindingsFile(t, dir, "audit.ndjson", []string{
		`{"role":"go-safety","item_id":"a"}`,
		``,
		`{"role":"go-safety","item_id":"b"}`,
		`{"role":"go-safety","item_id":"c"}`,
	})

	_, stderr, code := runTPMerge(t, dir, "audit", "--merge", "--json", rows)
	require.Equal(t, 1, code, "an input whose every row lacks status parses nothing: %s", stderr)

	for _, n := range []int{1, 3, 4} {
		assert.Contains(t, stderr, fmt.Sprintf("warning: skipping incomplete line (missing status) in %s:%d\n", rows, n),
			"each skip warning names its line")
	}
	msg, hint := mergeRefusal(t, stderr)
	assert.Contains(t, msg, rows+" (missing status: lines 1,3-4)", "the error names the field and the lines")
	assert.NotContains(t, hint, formatBlame, "the JSON was fine, so the hint must not blame its format")
	assert.Contains(t, hint, "item_id and status", "the hint names what every audit row needs")
}

// TestMerge_InvalidJSONRefusalKeepsTheFormatHint pins the case the old hint
// was written for, so the fix above cannot drop it: a file whose every line is
// broken JSON still gets told how to re-emit it, and gets no field advice.
func TestMerge_InvalidJSONRefusalKeepsTheFormatHint(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"review", "audit"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			bad := writeFindingsFile(t, dir, "bad.ndjson", []string{goodFinding1 + `,`, `{not json`})

			_, stderr, code := runTPMerge(t, dir, phase, "--merge", "--json", bad)
			require.Equal(t, 1, code, "an input of broken lines parses nothing: %s", stderr)

			assert.Contains(t, stderr, fmt.Sprintf("warning: skipping malformed line (invalid JSON) in %s:2\n", bad),
				"each skip warning names its line")
			msg, hint := mergeRefusal(t, stderr)
			assert.Contains(t, msg, bad+" (invalid JSON: lines 1-2)")
			assert.Contains(t, hint, formatBlame, "broken JSON still gets the format hint")
			assert.NotContains(t, hint, "needs a non-empty", "no row lacked a field")
		})
	}
}

// TestReviewMerge_EveryDroppedInputIsDiagnosedInOneExit: two inputs dropped for
// different reasons are both named, each with its own reasons and lines, and
// the hint carries the advice for both. An input that parsed a row is not
// dropped and is not in the error, though its skip still warns.
func TestReviewMerge_EveryDroppedInputIsDiagnosedInOneExit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	noEvidence := writeFindingsFile(t, dir, "a.ndjson", []string{
		`{"severity":"high","class":"gap","location":"§1","finding":"f1"}`,
	})
	mixed := writeFindingsFile(t, dir, "b.ndjson", []string{
		`{not json`,
		`{"severity":"high","class":"gap","finding":"f2"}`,
	})
	partial := writeFindingsFile(t, dir, "c.ndjson", []string{goodFinding1, `{also not json`})

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", noEvidence, mixed, partial)
	require.Equal(t, 1, code, "two inputs parsed nothing: %s", stderr)

	msg, hint := mergeRefusal(t, stderr)
	assert.Contains(t, msg, noEvidence+" (missing evidence: line 1)")
	assert.Contains(t, msg, mixed+" (invalid JSON: line 1; missing location, evidence: line 2)")
	assert.NotContains(t, msg, partial, "an input that parsed a row is not a dropped input")
	assert.Contains(t, stderr, fmt.Sprintf("in %s:2\n", partial), "its skipped line still warns, with its line")
	assert.Contains(t, hint, formatBlame, "one dropped line was broken JSON")
	assert.Contains(t, hint, "severity, finding, location and evidence", "and others lacked fields")
}

// TestReviewMerge_LongLineListIsCapped: the error stays compact however many
// lines were skipped — groups of consecutive lines are ranges, and past the
// cap the rest are counted rather than listed.
func TestReviewMerge_LongLineListIsCapped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// 30 rows on the odd lines 1..59, a blank line between each, so no two
	// skipped lines are consecutive and every one is its own group.
	lines := make([]string, 0, 59)
	for n := range 30 {
		if n > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf(`{"severity":"high","class":"gap","location":"§%d","finding":"f%d"}`, n, n))
	}
	rev := writeFindingsFile(t, dir, "rev.ndjson", lines)

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", rev)
	require.Equal(t, 1, code, "%s", stderr)
	require.Equal(t, 30, strings.Count(stderr, "warning: skipping incomplete line"), "every skip still warns")

	msg, _ := mergeRefusal(t, stderr)
	assert.Contains(t, msg, rev+" (missing evidence: lines 1,3,5,7,9,11,13,15 and 22 more)")
	assert.NotContains(t, msg, ",17", "lines past the cap are counted, not listed")
}

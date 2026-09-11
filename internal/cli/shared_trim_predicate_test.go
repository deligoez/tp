package cli_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergeSkipPattern extracts the key list out of `--merge`'s incomplete-line
// warning. The verdict of these tests rests on the SET of keys named, not on
// the exit code and not on any one key: a build that trims `evidence` and
// `location` while leaving `severity` and `finding` on an equality-to-empty
// test still skips the row and still exits 0, so a test asserting either is
// green under it.
var mergeSkipPattern = regexp.MustCompile(`skipping incomplete line \(missing ([^)]*)\)`)

// keysNamedBySkipWarning returns the keys every incomplete-line warning on
// stderr names, in the order they appear.
func keysNamedBySkipWarning(t *testing.T, stderr string) []string {
	t.Helper()
	out := make([]string, 0, 4)
	for _, m := range mergeSkipPattern.FindAllStringSubmatch(stderr, -1) {
		for k := range strings.SplitSeq(m[1], ",") {
			out = append(out, strings.TrimSpace(k))
		}
	}
	return out
}

// TestReviewGates_WhitespaceOnlyEvidenceIsMissingAtBothGates is §5 row 9: an
// `evidence` of one space is present and is a non-empty string, and both gates
// judge it missing.
//
// The two rows sit in one file so the file still parses its second row: that is
// what makes the merge verdict exit 0 with a warning rather than row 5's
// dropped input. Row 9's mutant is each gate keeping the emptiness test the
// shipped tree already gave it — `--merge`'s `x == ""` against `--record`'s
// `strings.TrimSpace(x) == ""` — under which `--merge` emits both rows with no
// warning while `--record` still refuses the first.
func TestReviewGates_WhitespaceOnlyEvidenceIsMissingAtBothGates(t *testing.T) {
	t.Parallel()
	dir := specWithOneRecordedRound(t)

	const blankEvidence = `{"role":"implementer","severity":"high","class":"gap","location":"§1","finding":"missing bound","evidence":" "}`
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{blankEvidence, goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)

	require.Equal(t, 0, code,
		"the file still parses its second row, so the skip is a warning rather than a dropped input: %s", stderr)
	assert.Equal(t, []string{"evidence"}, keysNamedBySkipWarning(t, stderr),
		"the warning names `evidence` and nothing else: the other three keys are legal")

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{1, 1}, inputs[f1], "the whitespace-only evidence row is skipped, its neighbour parsed")

	// The other half of row 9: the same two rows at the record gate.
	_, stderr, code = recordRound(t, dir, blankEvidence+"\n"+goodFinding3+"\n")
	require.Equal(t, 1, code, "--record refuses the whitespace-only evidence row: %s", stderr)
	assert.Equal(t, []int{1}, linesNamed(t, stderr), "--record names the offending line and only it")
}

// TestReviewMerge_WhitespaceOnlyLocationIsMissing is §5 row 11: a `location` of
// one space is skipped by `--merge` exactly as an absent `location` is.
//
// Its mutant is `HEAD`'s `missingFindingFields`, which tests `x == ""`: the
// file parses two rows, no warning is written, and `--merge` exits 0 — so the
// exit code alone does not separate the two and the assertion is on the
// warning and on the per-input counts.
func TestReviewMerge_WhitespaceOnlyLocationIsMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	const blankLocation = `{"role":"implementer","severity":"high","class":"gap","location":" ","finding":"missing bound","evidence":"read §1 and ran tp lint"}`
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{blankLocation, goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)

	require.Equal(t, 0, code,
		"the file still parses its second row, so the skip is a warning rather than a dropped input: %s", stderr)
	assert.Equal(t, []string{"location"}, keysNamedBySkipWarning(t, stderr),
		"a whitespace-only location is named exactly as a missing one is")

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{1, 1}, inputs[f1], "the whitespace-only location row is skipped, its neighbour parsed")
}

// TestReviewMerge_TrimDecidesEmptinessForEveryRequiredKey pins the predicate
// over all four keys rather than two. A build trimming only `evidence` and
// `location` while leaving `severity` and `finding` on an equality-to-empty
// test was measured green across every other §5 row — it still skips this
// row, still warns and still exits 0 — so the fixture carries a single space
// in each of `severity`, `location` and `finding` at once and the verdict is
// the SET of keys the warning names. The test goes red when the trim reaches
// fewer than four keys.
func TestReviewMerge_TrimDecidesEmptinessForEveryRequiredKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	const blankThree = `{"role":"implementer","severity":" ","class":"gap","location":" ","finding":" ","evidence":"read §1 and ran tp lint"}`
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{blankThree, goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)

	require.Equal(t, 0, code, "the legal second row keeps the input from being dropped: %s", stderr)
	assert.ElementsMatch(t, []string{"severity", "finding", "location"}, keysNamedBySkipWarning(t, stderr),
		"trim decides emptiness for severity and finding as much as for location and evidence")

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{1, 1}, inputs[f1], "the row is skipped whichever key is read first")
}

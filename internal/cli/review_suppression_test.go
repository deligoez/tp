package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextRoundAction is next_action's run-the-next-round branch for these fixtures.
// It is what the state falls through to once every candidate is mechanized.
const nextRoundAction = "run the next review round: tp review spec.md --record <file>"

// suppressionFixture creates a spec plus a resolved task file in a fresh temp
// dir and registers checksJSON as the workflow's `checks` when non-empty.
func suppressionFixture(t *testing.T, checksJSON string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	if checksJSON != "" {
		_, stderr, code = runTP(t, dir, "set", "--workflow", "checks="+checksJSON)
		require.Equal(t, 0, code, "registering checks failed: %s", stderr)
	}
	return dir
}

// fiveRowsOfClass returns five low-severity finding rows of one class, each at
// its own location, so the class crosses the "at least 5 times in one round"
// half of the candidate threshold on its own. One round keeps the fixture below
// required_clean_rounds, so next_action reaches its mechanize branch instead of
// the converged one.
func fiveRowsOfClass(class string) []string {
	const threshold = 5
	rows := make([]string, 0, threshold)
	for i := range threshold {
		rows = append(rows, classRow(
			fmt.Sprintf("L%d-%s", i, class),
			fmt.Sprintf("finding %d of %s", i, class),
			class))
	}
	return rows
}

// recordSuppressionRound records rows as one review round and returns the raw
// stdout of `tp review spec.md --record`.
func recordSuppressionRound(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	f := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(strings.Join(rows, "\n")+"\n"), 0o600))
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", f)
	require.Equal(t, 0, code, "record failed: %s", stderr)
	return stdout
}

// candidateClasses projects a --record payload's mechanize_candidates to its
// class strings, preserving the emitted order.
func candidateClasses(t *testing.T, raw string) []string {
	t.Helper()
	var payload struct {
		MechanizeCandidates []struct {
			Class string `json:"class"`
		} `json:"mechanize_candidates"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	classes := make([]string, 0, len(payload.MechanizeCandidates))
	for _, c := range payload.MechanizeCandidates {
		classes = append(classes, c.Class)
	}
	return classes
}

// mechanizedClasses reads a --record payload's mechanized_classes array.
func mechanizedClasses(t *testing.T, raw string) []string {
	t.Helper()
	var payload struct {
		MechanizedClasses []string `json:"mechanized_classes"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	return payload.MechanizedClasses
}

// Test 32: a registered check suppresses its class from mechanize_candidates on
// --record, while an unregistered class over the same threshold stays listed —
// the observable form of "suppressing one class never changes whether another
// crosses the frequency threshold" (§3.2).
func TestReviewRecord_RegisteredCheckSuppressesItsClass(t *testing.T) {
	t.Parallel()
	dir := suppressionFixture(t, `[{"class":"registered-class","cmd":"true"}]`)
	rows := append(fiveRowsOfClass("registered-class"), fiveRowsOfClass("unregistered-class")...)

	classes := candidateClasses(t, recordSuppressionRound(t, dir, rows...))
	assert.Equal(t, []string{"unregistered-class"}, classes,
		"the registered class is withheld; the unregistered one over the same threshold is still listed")
}

// Test 33: the class match is byte-exact, and the varying side is the finding
// row's class. Varying the registered side would prove nothing — ValidateChecks
// rejects both variants, so §3.1's validity rule alone would leave the candidate
// listed and a trimming or case-folding implementation would pass unchanged.
func TestReviewRecord_SuppressionMatchesTheFindingClassExactly(t *testing.T) {
	t.Parallel()
	dir := suppressionFixture(t, `[{"class":"duplicate-line","cmd":"true"}]`)
	rows := fiveRowsOfClass("duplicate-line")
	rows = append(rows, fiveRowsOfClass("Duplicate-Line")...)
	rows = append(rows, fiveRowsOfClass(" duplicate-line ")...)

	classes := candidateClasses(t, recordSuppressionRound(t, dir, rows...))
	assert.ElementsMatch(t, []string{"Duplicate-Line", " duplicate-line "}, classes,
		"a case- or whitespace-variant candidate class is not what the registered check covers")
	assert.NotContains(t, classes, "duplicate-line", "the byte-exact class is suppressed")
}

// Test 38: suppressing every candidate removes the register-a-check hint from
// --record output, and next_action on that same invocation names no mechanized
// class — mode 1's third sink, which an implementation filtering only the
// emitted array and the hint would leave unfiltered. The unregistered control
// proves the fixture really does reach next_action's mechanize branch.
func TestReviewRecord_SuppressingEveryCandidateDropsHintAndNextActionClass(t *testing.T) {
	t.Parallel()
	control := suppressionFixture(t, "")
	controlOut := recordSuppressionRound(t, control, fiveRowsOfClass("only-class")...)
	var controlPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(controlOut), &controlPayload))
	require.Contains(t, controlPayload, "hint", "unregistered: the register-a-check hint fires")
	require.Contains(t, controlPayload["next_action"], `"only-class"`,
		"unregistered: next_action names the candidate class")

	dir := suppressionFixture(t, `[{"class":"only-class","cmd":"true"}]`)
	stdout := recordSuppressionRound(t, dir, fiveRowsOfClass("only-class")...)

	assert.Contains(t, stdout, `"mechanize_candidates": []`,
		"the filtered array stays an emitted empty array, asserted against the raw JSON")
	assert.NotContains(t, stdout, `"mechanize_candidates": null`,
		"a filter appending survivors into a nil slice would emit null here")

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	assert.NotContains(t, payload, "hint",
		"the register-a-check hint is already conditional on a non-empty candidate list")
	assert.Equal(t, nextRoundAction, payload["next_action"],
		"the state falls through to the run-the-next-round branch")
}

// Test 39: `tp review --status` honours the suppression through next_action on
// the same fixture, with neither mechanize_candidates nor mechanized_classes in
// its output. This is the second sink: --status emits no candidate array of its
// own and derives its class list from the recorded rounds by a separate call, so
// filtering mode 1's array alone leaves it naming a class whose check exists.
func TestReviewStatus_SuppressionReachesNextAction(t *testing.T) {
	t.Parallel()
	control := suppressionFixture(t, "")
	recordSuppressionRound(t, control, fiveRowsOfClass("only-class")...)
	controlOut, stderr, code := runTP(t, control, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "status failed: %s", stderr)
	var controlPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(controlOut), &controlPayload))
	require.Contains(t, controlPayload["next_action"], `"only-class"`,
		"unregistered: --status names the candidate class through its own call")

	dir := suppressionFixture(t, `[{"class":"only-class","cmd":"true"}]`)
	recordSuppressionRound(t, dir, fiveRowsOfClass("only-class")...)
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "status failed: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	assert.NotContains(t, payload, "mechanize_candidates", "--status carries no candidate array")
	assert.NotContains(t, payload, "mechanized_classes", "--status carries nothing for it to explain")
	assert.Equal(t, nextRoundAction, payload["next_action"])
}

// Test 40: whether the registered check passes is irrelevant — registration is
// the trigger. Asserting only on --record would prove nothing, since that mode
// executes no check and a failing entry is indistinguishable from a passing one
// there; `--status --check` is the mode that actually runs it.
func TestReviewSuppression_FailingCheckStillSuppressesItsClass(t *testing.T) {
	t.Parallel()
	dir := suppressionFixture(t, `[{"class":"failing-class","cmd":"echo tail-marker; exit 1"}]`)
	recordOut := recordSuppressionRound(t, dir, fiveRowsOfClass("failing-class")...)
	assert.Empty(t, candidateClasses(t, recordOut), "--record withholds the class without running the check")
	assert.Equal(t, []string{"failing-class"}, mechanizedClasses(t, recordOut),
		"and names it in mechanized_classes: registration is the trigger, not the check's exit status")

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
	require.Equal(t, 1, code, "a failing mechanical check still gates the exit code: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	checks := payload["mechanical_checks"].([]any)
	require.Len(t, checks, 1)
	entry := checks[0].(map[string]any)
	assert.Equal(t, "failing-class", entry["class"])
	assert.Equal(t, false, entry["passed"], "the failure is still reported in mechanical_checks")
	assert.Contains(t, entry["output_tail"], "tail-marker")
	assert.Equal(t, nextRoundAction, payload["next_action"],
		"a failing check suppresses its class from next_action exactly as a passing one does")
}

// Test 45: `tp review --report` is unchanged (Non-Goal 9 of v0.33.0, the spec
// this file's section and test numbers refer to) — a class registered in a
// resolvable task file still appears in its mechanize_candidates, and the
// report emits no mechanized_classes key.
func TestReviewReport_SuppressionDoesNotReachReport(t *testing.T) {
	t.Parallel()
	dir := suppressionFixture(t, `[{"class":"registered-class","cmd":"true"}]`)
	f := filepath.Join(dir, "r1.ndjson")
	rows := fiveRowsOfClass("registered-class")
	require.NoError(t, os.WriteFile(f, []byte(strings.Join(rows, "\n")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "--report", f)
	require.Equal(t, 0, code, "report failed: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	candidates := payload["mechanize_candidates"].([]any)
	require.Len(t, candidates, 1, "--report suppresses nothing")
	assert.Equal(t, "registered-class", candidates[0].(map[string]any)["class"])
	assert.NotContains(t, payload, "mechanized_classes")
}

// Test 41's ordering and intersection halves: mechanized_classes lists the
// withheld classes sorted ascending, and lists nothing else. The three withheld
// classes are registered out of alphabetical order AND carry different totals,
// so ascending order differs from both registration order and the total-descending
// candidate order the control fixture pins — an implementation projecting the
// filter's input order into the field fails on the first assertion. rare-class is
// registered but never reached candidate frequency, which the control proves by
// leaving it out of an unfiltered candidate list, so the filter never saw it and
// the field does not name it.
func TestReviewRecord_MechanizedClassesIsSortedAndListsTheIntersection(t *testing.T) {
	t.Parallel()
	rows := fiveRowsOfClass("alpha-class")
	rows = append(rows, fiveRowsOfClass("mid-class")...)
	rows = append(rows, classRow("L5-mid-class", "sixth of mid-class", "mid-class"))
	rows = append(rows, fiveRowsOfClass("zeta-class")...)
	rows = append(rows,
		classRow("L5-zeta-class", "sixth of zeta-class", "zeta-class"),
		classRow("L6-zeta-class", "seventh of zeta-class", "zeta-class"))
	rows = append(rows, fiveRowsOfClass("control-class")...)
	rows = append(rows, classRow("L9-rare", "seen once", "rare-class"))

	control := suppressionFixture(t, "")
	assert.Equal(t, []string{"zeta-class", "mid-class", "alpha-class", "control-class"},
		candidateClasses(t, recordSuppressionRound(t, control, rows...)),
		"unfiltered: candidates run total-descending, and rare-class never reaches candidate frequency")

	dir := suppressionFixture(t, `[{"class":"zeta-class","cmd":"true"},{"class":"alpha-class","cmd":"true"},`+
		`{"class":"mid-class","cmd":"true"},{"class":"rare-class","cmd":"true"}]`)
	stdout := recordSuppressionRound(t, dir, rows...)

	assert.Equal(t, []string{"alpha-class", "mid-class", "zeta-class"}, mechanizedClasses(t, stdout),
		"the withheld classes, each once and sorted ascending")
	assert.Equal(t, []string{"control-class"}, candidateClasses(t, stdout),
		"the survivors keep their own order beside it")
}

// Test 41's shape half. Both arrays are asserted against the raw JSON rather
// than through a decoder, which renders an absent key, [] and null alike: the
// null assertions are the discriminating ones against the nil-slice
// serialization this repository records as a recurring defect. Two fixtures are
// needed because the two empty cases are opposite rounds — nothing withheld
// empties mechanized_classes, everything withheld empties mechanize_candidates.
func TestReviewRecord_BothArraysStayEmittedEmptyArrays(t *testing.T) {
	t.Parallel()
	nothingWithheld := recordSuppressionRound(t, suppressionFixture(t, ""), fiveRowsOfClass("only-class")...)
	assert.Contains(t, nothingWithheld, `"mechanized_classes": []`,
		"nothing withheld emits an empty array, not an absent key")
	assert.NotContains(t, nothingWithheld, `"mechanized_classes": null`,
		"a withheld set collected into a nil slice would emit null here")
	assert.NotEmpty(t, candidateClasses(t, nothingWithheld), "the round really does have a candidate")

	everythingWithheld := recordSuppressionRound(t,
		suppressionFixture(t, `[{"class":"only-class","cmd":"true"}]`), fiveRowsOfClass("only-class")...)
	assert.Contains(t, everythingWithheld, `"mechanize_candidates": []`,
		"the filter emptying the candidate list keeps it an emitted empty array")
	assert.NotContains(t, everythingWithheld, `"mechanize_candidates": null`,
		"a filter appending survivors into a nil slice would emit null here")
	assert.Equal(t, []string{"only-class"}, mechanizedClasses(t, everythingWithheld))
}

// Test 41's --compact half: mechanized_classes survives --compact because
// mechanize_candidates does, since stripping one half of a list and its withheld
// remainder would misreport the round rather than shorten it. The absent
// harness_stale key is what proves --compact actually took effect on this
// payload.
func TestReviewRecord_MechanizedClassesSurvivesCompact(t *testing.T) {
	t.Parallel()
	dir := suppressionFixture(t, `[{"class":"registered-class","cmd":"true"}]`)
	rows := append(fiveRowsOfClass("registered-class"), fiveRowsOfClass("unregistered-class")...)
	f := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(strings.Join(rows, "\n")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", f, "--compact")
	require.Equal(t, 0, code, "compact record failed: %s", stderr)
	assert.NotContains(t, stdout, "harness_stale", "--compact took effect on this payload")

	assert.Equal(t, []string{"registered-class"}, mechanizedClasses(t, stdout))
	assert.Equal(t, []string{"unregistered-class"}, candidateClasses(t, stdout))
}

// Test 35's mechanized_classes half: a class named by two valid entries and over
// the candidate threshold is mechanized and named once. The frequency
// precondition is what makes this half assertable, since the array holds
// withheld candidates. The fixture writes the block directly because
// `tp set --workflow checks=` validates the whole slice and rejects the
// duplicate class this test needs.
func TestReviewRecord_ClassNamedByTwoEntriesWithheldOnce(t *testing.T) {
	t.Parallel()
	dir := exclusionFixture(t, `[{"class":"twice-class","cmd":"check-a"},{"class":"twice-class","cmd":"check-b"}]`)
	stdout := recordSuppressionRound(t, dir, fiveRowsOfClass("twice-class")...)

	assert.Equal(t, []string{"twice-class"}, mechanizedClasses(t, stdout), "named once, not once per entry")
	assert.Empty(t, candidateClasses(t, stdout), "and withheld from the candidate list")
}

// Test 36's mechanized_classes half: over-specification is withheld and listed
// like any other class. Its §3.1 exemption is scoped to the reviewer exclusion
// list alone — an implementation exempting it here too would leave the
// register-a-check hint firing for a class whose check already exists.
func TestReviewRecord_OverSpecificationIsWithheldLikeAnyOtherClass(t *testing.T) {
	t.Parallel()
	dir := suppressionFixture(t, `[{"class":"over-specification","cmd":"true"}]`)
	rows := append(fiveRowsOfClass("over-specification"), fiveRowsOfClass("control-class")...)
	stdout := recordSuppressionRound(t, dir, rows...)

	assert.Equal(t, []string{"over-specification"}, mechanizedClasses(t, stdout))
	assert.Equal(t, []string{"control-class"}, candidateClasses(t, stdout),
		"the exemption does not reach this sink")
}

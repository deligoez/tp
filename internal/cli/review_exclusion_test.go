package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exclusionMarker is the §3.2 sentence prefix. What follows it, up to the end
// of the line, is the reviewer exclusion list.
const exclusionMarker = "Mechanically checked classes — do NOT report findings of these classes: "

// exclusionFixture creates a spec plus a resolved task file in a fresh temp dir
// and registers checksJSON as the workflow's `checks`.
//
// The entries these tests need include ones ValidateChecks rejects, and
// `tp set --workflow checks=` validates before writing, so the block is written
// into the resolved task file directly. A resolved task file is also what §3.2
// requires of the fixture: without one the mechanical-check runner returns
// before its loop and emits no `skipping invalid check` notice, which is the
// observable the all-invalid case turns on.
//
// The dir also carries the baseline and findings file the standalone regression
// site takes, so both prompt-emission sites can be driven from one fixture.
func exclusionFixture(t *testing.T, checksJSON string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## One\ncontent\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)

	path := filepath.Join(dir, "spec.tasks.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(raw, &tf))
	var checks any
	require.NoError(t, json.Unmarshal([]byte(checksJSON), &checks))
	tf["workflow"] = map[string]any{"checks": checks}
	patched, err := json.MarshalIndent(tf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, patched, 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "baseline.md"), []byte("# Spec\n\n## One\nolder content\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "regression-findings.ndjson"),
		[]byte(`{"severity":"low","category":"consistency","location":"L1","finding":"f","suggestion":"fix","resolved":{"status":"fixed","evidence":"e"}}`+"\n"), 0o600))
	return dir
}

// promptTexts projects a review payload to its prompt bodies, in emitted order.
func promptTexts(t *testing.T, stdout string) []string {
	t.Helper()
	var payload struct {
		Prompts []struct {
			Prompt string `json:"prompt"`
		} `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.NotEmpty(t, payload.Prompts, "prompt emission produced no prompt")
	texts := make([]string, 0, len(payload.Prompts))
	for _, p := range payload.Prompts {
		texts = append(texts, p.Prompt)
	}
	return texts
}

// exclusionClasses returns the classes the §3.2 sentence names in text, and
// false when text carries no such sentence at all — the empty-list case, where
// the sentence is not appended rather than appended with nothing after it.
func exclusionClasses(t *testing.T, text string) ([]string, bool) {
	t.Helper()
	_, after, ok := strings.Cut(text, exclusionMarker)
	if !ok {
		assert.NotContains(t, text, "Mechanically checked classes",
			"an empty list means no sentence at all, not one ending in an empty list")
		return nil, false
	}
	rest := after
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[:nl]
	}
	return strings.Split(rest, ", "), true
}

// panelPrompts drives the multi-role panel emission site.
func panelPrompts(t *testing.T, dir string) (texts []string, stderr string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "panel review failed: %s", stderr)
	return promptTexts(t, stdout), stderr
}

// regressionPrompt drives the standalone `tp review <spec> --perspective
// regression` site in its explicit --diff-from plus --findings mode.
func regressionPrompt(t *testing.T, dir string) (text, stderr string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression",
		"--diff-from", "baseline.md", "--findings", "regression-findings.ndjson")
	require.Equal(t, 0, code, "standalone regression failed: %s", stderr)
	texts := promptTexts(t, stdout)
	require.Len(t, texts, 1, "the regression perspective emits one prompt")
	return texts[0], stderr
}

// assertExclusionAtBothSites pins the same list at both §3.2 sites. The
// regression half is the discriminating one: an implementation that filters
// only the panel path passes the first assertion and fails this one.
func assertExclusionAtBothSites(t *testing.T, dir string, want []string) {
	t.Helper()
	texts, _ := panelPrompts(t, dir)
	for i, text := range texts {
		got, ok := exclusionClasses(t, text)
		require.True(t, ok, "panel prompt %d carries the sentence", i)
		assert.Equal(t, want, got, "panel prompt %d", i)
	}

	reg, _ := regressionPrompt(t, dir)
	got, ok := exclusionClasses(t, reg)
	require.True(t, ok, "the standalone regression site carries the sentence")
	assert.Equal(t, want, got, "standalone regression site")
}

// Test 42's invalid-entry clause, at both of test 44's sites: prompt emission
// drops an entry the validator rejects while keeping a valid one. Validity is
// judged per entry, so an implementation validating the whole slice would drop
// both. Test 34 states that same per-entry rule at the candidate sink rather
// than at this one; the predicate the two sinks share is pinned in package
// engine by TestIsMechanizedClass_ValidityIsPerEntry.
func TestReviewExclusion_InvalidEntryDroppedAtBothSites(t *testing.T) {
	dir := exclusionFixture(t, `[{"class":"Bad_Slug","cmd":"true"},{"class":"kept-class","cmd":"true"}]`)
	assertExclusionAtBothSites(t, dir, []string{"kept-class"})
}

// Test 42's ordering clause: over three valid entries registered out of
// alphabetical order the list keeps registration order, which is what
// distinguishes it from the ascending `mechanized_classes` of §3.3.
func TestReviewExclusion_KeepsRegistrationOrder(t *testing.T) {
	dir := exclusionFixture(t, `[{"class":"zeta-check","cmd":"true"},{"class":"alpha-check","cmd":"true"},{"class":"mid-check","cmd":"true"}]`)
	assertExclusionAtBothSites(t, dir, []string{"zeta-check", "alpha-check", "mid-check"})
}

// Test 42's frequency clause: a registered class that never reached candidate
// frequency stays on the list, because the list is the mechanized set and not
// the suppressed-candidate set. The control fixture — the same rows with
// nothing registered — is what proves rare-class really is below the candidate
// threshold, so an implementation deriving this list from the withheld
// candidates would list frequent-class alone and fail.
func TestReviewExclusion_KeepsAClassBelowCandidateFrequency(t *testing.T) {
	rows := append(fiveRowsOfClass("frequent-class"), classRow("L9", "seen once", "rare-class"))

	control := suppressionFixture(t, "")
	assert.Equal(t, []string{"frequent-class"}, candidateClasses(t, recordSuppressionRound(t, control, rows...)),
		"rare-class never reaches candidate frequency; frequent-class does")

	dir := exclusionFixture(t, `[{"class":"frequent-class","cmd":"true"},{"class":"rare-class","cmd":"true"}]`)
	recordSuppressionRound(t, dir, rows...)
	assertExclusionAtBothSites(t, dir, []string{"frequent-class", "rare-class"})
}

// Test 37: filter order. With an invalid entry and a valid entry naming the same
// class, that class is mechanized and appears on the list. An implementation
// collapsing duplicates before dropping invalid entries keeps the invalid
// occurrence, drops the class, and fails.
func TestReviewExclusion_InvalidEntryDoesNotShadowAValidOne(t *testing.T) {
	dir := exclusionFixture(t, `[{"class":"dup-class","cmd":"   "},{"class":"dup-class","cmd":"true"}]`)
	assertExclusionAtBothSites(t, dir, []string{"dup-class"})
}

// Test 35's exclusion-list half at prompt emission: a class named by two valid
// entries is registered, not rejected, and the list names it once.
func TestReviewExclusion_ClassNamedByTwoEntriesListedOnce(t *testing.T) {
	dir := exclusionFixture(t, `[{"class":"twice-class","cmd":"check-a"},{"class":"twice-class","cmd":"check-b"}]`)
	assertExclusionAtBothSites(t, dir, []string{"twice-class"})
}

// Test 36's exclusion-list half at both sites: over-specification never joins
// the list, while a valid entry beside it does. The second assertion is the
// reason — the same prompt's canonical-class instruction tells the reviewer to
// raise the class, so listing it would ship one prompt that both demands and
// forbids the same finding.
func TestReviewExclusion_OverSpecificationNeverJoinsTheList(t *testing.T) {
	dir := exclusionFixture(t, `[{"class":"over-specification","cmd":"true"},{"class":"naming-class","cmd":"true"}]`)
	assertExclusionAtBothSites(t, dir, []string{"naming-class"})

	texts, _ := panelPrompts(t, dir)
	assert.Contains(t, texts[0], "Canonical class `over-specification`",
		"the prompt still tells the reviewer to raise the class it must not be told to suppress")
}

// Test 43: when every registered entry is invalid, prompt emission appends no
// sentence at all at either site, while each entry's `skipping invalid check`
// notice is still emitted. The notice is the assertion that can fail —
// mechanical_checks is omitted under omitempty whether the runner ran and
// returned nothing or was never called — and it is also what proves the
// surrounding checks-running branch kept its own guard.
func TestReviewExclusion_EveryEntryInvalidAppendsNoSentence(t *testing.T) {
	dir := exclusionFixture(t, `[{"class":"Bad_Slug","cmd":"true"},{"class":"blank-cmd","cmd":"   "}]`)

	texts, panelStderr := panelPrompts(t, dir)
	for i, text := range texts {
		_, ok := exclusionClasses(t, text)
		assert.False(t, ok, "panel prompt %d appends no sentence", i)
	}
	assertInvalidCheckNotices(t, panelStderr)

	reg, regressionStderr := regressionPrompt(t, dir)
	_, ok := exclusionClasses(t, reg)
	assert.False(t, ok, "the standalone regression site appends no sentence either")
	assertInvalidCheckNotices(t, regressionStderr)
}

func assertInvalidCheckNotices(t *testing.T, stderr string) {
	t.Helper()
	assert.Contains(t, stderr, "skipping invalid check 0 (Bad_Slug)",
		"the mechanical checks still run over every entry")
	assert.Contains(t, stderr, "skipping invalid check 1 (blank-cmd)")
}

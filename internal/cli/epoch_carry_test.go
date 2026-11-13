package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emitReview emits the next review round and returns its payload.
func emitReview(t *testing.T, dir string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emit: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out
}

// promptsCarrying counts the emitted prompts holding marker.
func promptsCarrying(out map[string]any, marker string) int {
	n := 0
	for _, p := range out["prompts"].([]any) {
		if strings.Contains(p.(map[string]any)["prompt"].(string), marker) {
			n++
		}
	}
	return n
}

// epochFixedRow is a finding about the old text that round 1 records and the
// fixture then resolves fixed: the regression pass lists it as a decision to
// guard.
const epochFixedRow = `{"evidence":"line 5","severity":"high","category":"clarity","location":"§1","finding":"MARKER-FIXED-OLD widget colour was undefined","suggestion":"s"}` + "\n"

// fixedRewrittenFixture records round 1 with epochFixedRow over the old text,
// resolves it fixed, then replaces the spec.
func fixedRewrittenFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	_, stderr, code := recordRound(t, dir, epochFixedRow)
	require.Equal(t, 0, code, "record round 1: %s", stderr)
	round1 := filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson")
	_, stderr, code = runTP(t, dir, "review", round1, "--resolve", "0", "fixed", "said blue")
	require.Equal(t, 0, code, "resolve: %s", stderr)
	writeSpec(t, dir, epochNewSpec)
	return dir
}

// regressionPrompt returns the emitted regression prompt, or "".
func emittedRegressionPrompt(out map[string]any) string {
	for _, p := range out["prompts"].([]any) {
		if m := p.(map[string]any); m["role"] == "regression" {
			return m["prompt"].(string)
		}
	}
	return ""
}

func startEpoch(t *testing.T, dir string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--rewritten", "replaced the spec wholesale")
	require.Equal(t, 0, code, "--rewritten: %s", stderr)
}

// TestEpoch_TheReviewCarryListStartsAtTheEpoch is the field report's repro: a
// rewritten spec's round-2 prompts carried round 1's finding about the old
// text under "DO NOT re-report". After --rewritten they carry none of it, and
// the same history without the epoch still does.
func TestEpoch_TheReviewCarryListStartsAtTheEpoch(t *testing.T) {
	t.Parallel()
	without := rewrittenFixture(t)
	assert.Positive(t, promptsCarrying(emitReview(t, without), "MARKER-OLD-TEXT"),
		"without an epoch round 2 carries round 1's row")

	dir := rewrittenFixture(t)
	startEpoch(t, dir)
	out := emitReview(t, dir)
	assert.Zero(t, promptsCarrying(out, "MARKER-OLD-TEXT"), "after the epoch no prompt carries the old text's row")
	assert.Equal(t, float64(0), out["review_loop"].(map[string]any)["previous_findings"])
	assert.Equal(t, float64(2), out["review_loop"].(map[string]any)["round"], "the round number still counts every round")
}

// TestEpoch_TheRegressionPassListsNoFixedRowFromBeforeTheEpoch: the regression
// pass guards findings resolved fixed; one fixed in the old text is no decision
// of the new one, so after the epoch the auto-appended regression prompt does
// not list it, while the same history without the epoch does.
func TestEpoch_TheRegressionPassListsNoFixedRowFromBeforeTheEpoch(t *testing.T) {
	t.Parallel()
	without := fixedRewrittenFixture(t)
	require.Contains(t, emittedRegressionPrompt(emitReview(t, without)), "MARKER-FIXED-OLD",
		"without an epoch the regression pass lists round 1's fixed row")

	dir := fixedRewrittenFixture(t)
	startEpoch(t, dir)
	prompt := emittedRegressionPrompt(emitReview(t, dir))
	require.NotEmpty(t, prompt, "the rewrite still changed every section, so the pass is still appended")
	assert.NotContains(t, prompt, "MARKER-FIXED-OLD")
}

// TestEpoch_StandaloneRegressionListsNoFixedRowFromBeforeTheEpoch: the same
// cut in `--perspective regression`, which reads the same list.
func TestEpoch_StandaloneRegressionListsNoFixedRowFromBeforeTheEpoch(t *testing.T) {
	t.Parallel()
	dir := fixedRewrittenFixture(t)
	startEpoch(t, dir)
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression")
	require.Equal(t, 0, code, stderr)
	assert.NotContains(t, stdout, "MARKER-FIXED-OLD")
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(0), out["review_loop"].(map[string]any)["previous_findings"])
}

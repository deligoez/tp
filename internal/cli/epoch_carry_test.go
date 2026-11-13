package cli_test

import (
	"encoding/json"
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

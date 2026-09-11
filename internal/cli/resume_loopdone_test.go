package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResume_AReviewTheCapEndedMovesOn: tp resume reads the same verdict import
// does. Three rounds reach the default cap; while round 3's finding is open the
// oracle stops on the cap, and once it carries a disposition the phase moves
// to decompose — where import already accepts the spec — instead of escalating.
func TestResume_AReviewTheCapEndedMovesOn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	for range 3 {
		_, stderr, code = recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code, stderr)
	}

	resume := func() map[string]any {
		stdout, stderr, code := runTP(t, dir, "resume", "--json")
		require.Equal(t, 0, code, stderr)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		return out
	}
	blockerCodes := func(out map[string]any) []string {
		codes := []string{}
		bs, _ := out["blockers"].([]any)
		for _, b := range bs {
			if m, ok := b.(map[string]any); ok {
				codes = append(codes, m["code"].(string))
			}
		}
		return codes
	}

	open := resume()
	assert.Equal(t, "review", open["phase"])
	assert.Contains(t, blockerCodes(open), "review-budget-exhausted", "an open finding at the cap stops the oracle")

	round3 := filepath.Join(".tp-review", "spec", "review-round-3.ndjson")
	_, stderr, code = runTP(t, dir, "review", round3, "--resolve", "0", "wontfix", "accepted by the operator")
	require.Equal(t, 0, code, stderr)

	ended := resume()
	assert.Equal(t, "decompose", ended["phase"], "the cap ended review, so the oracle moves on")
	assert.NotContains(t, blockerCodes(ended), "review-budget-exhausted")
}

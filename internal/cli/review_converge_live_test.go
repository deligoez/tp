package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mediumRow carries a below-blocking (medium) severity: it dirties the round
// at record time (so the frozen ReviewRound.Clean is false), yet under
// review_converge_on=blocking no surviving finding blocks, so the live
// severity-aware predicate treats the round as clean. This is precisely the
// case where the frozen flag and the live recompute disagree.
const mediumRow = `{"severity":"medium","category":"c","location":"L1","finding":"nit","suggestion":"s"}` + "\n"

// TestReviewConvergeLive_BudgetAndPromptGen asserts that, for a
// review_converge_on=blocking sequence whose only survivors are medium/low, the
// review budget refusal and prompt-generation convergence both treat the
// sequence as converged — matching tp review --status.
func TestReviewConvergeLive_BudgetAndPromptGen(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", "review_max_rounds=2", "review_converge_on=blocking")
	require.Equal(t, 0, code)

	// Two rounds, each with a single medium (non-blocking) survivor. Each is
	// frozen not-clean but live-clean under blocking; two of them satisfy the
	// default required_clean_rounds=2.
	for i := range 2 {
		_, stderr, c := recordRound(t, dir, mediumRow)
		require.Equal(t, 0, c, "round %d: %s", i+1, stderr)
	}

	// Budget refusal: at the cap but converged under the live predicate, so
	// prompt generation is NOT refused (exit 0, not exit 4).
	stdout, stderr, c := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, c, "converged medium/low sequence must not be budget-refused: %s", stderr)

	// Prompt-gen convergence: review_loop.converged / consecutive_clean are the
	// live severity-aware values.
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	loop, ok := out["review_loop"].(map[string]any)
	require.True(t, ok, "review_loop present")
	assert.Equal(t, true, loop["converged"], "prompt-gen convergence uses the live severity-aware predicate")
	assert.Equal(t, float64(2), loop["consecutive_clean"], "both medium/low rounds count as clean")

	// --status agrees with the prompt-gen and budget view.
	sOut, _, sc := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, sc)
	var st map[string]any
	require.NoError(t, json.Unmarshal([]byte(sOut), &st))
	assert.Equal(t, true, st["converged"], "--status converged")
	assert.Equal(t, false, st["budget_exhausted"], "--status budget_exhausted false when live-converged")
}

// TestReviewConvergeLive_ImportEnforcement asserts the review-convergence import
// gate uses the live severity-aware predicate: two medium/low-only blocking
// rounds import cleanly, consistent with tp review --status/--record.
func TestReviewConvergeLive_ImportEnforcement(t *testing.T) {
	dir := setupEnforceProject(t)

	for i := range 2 {
		_, stderr, code := recordRound(t, dir, mediumRow)
		require.Equal(t, 0, code, "round %d: %s", i+1, stderr)
	}

	stderr, code := importBare(t, dir)
	assert.Equal(t, 0, code, "medium/low-only blocking rounds count as converged for import: %s", stderr)
}

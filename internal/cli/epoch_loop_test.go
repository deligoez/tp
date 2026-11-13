package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewStatus(t *testing.T, dir string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out
}

// TestEpoch_ConvergenceAndTheCapStillCountEveryRound: an epoch changes prompts
// only. Convergence still reads the trailing clean rounds, the epoch's earlier
// ones included, and the round cap (3 by default) still counts every recorded
// round, so neither the streak nor the budget restarts at the epoch.
func TestEpoch_ConvergenceAndTheCapStillCountEveryRound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	for range 2 {
		_, stderr, code := recordRound(t, dir, "")
		require.Equal(t, 0, code, stderr)
	}
	writeSpec(t, dir, epochNewSpec)
	startEpoch(t, dir)

	st := reviewStatus(t, dir)
	assert.Equal(t, float64(2), st["consecutive_clean"], "the streak keeps the rounds before the epoch")
	assert.Equal(t, true, st["stale"], "the epoch does not clear spec-stale")
	assert.Equal(t, false, st["converged"])
	assert.Equal(t, float64(1), st["rounds_remaining"], "the cap counts the rounds before the epoch")

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emit round 3: %s", stderr)
	_, stderr, code = recordRound(t, dir, "")
	require.Equal(t, 0, code, stderr)

	st = reviewStatus(t, dir)
	assert.Equal(t, float64(3), st["consecutive_clean"])
	assert.Equal(t, false, st["stale"], "a round that read the new text clears staleness")
	assert.Equal(t, true, st["converged"])
	assert.Equal(t, float64(0), st["rounds_remaining"])
}


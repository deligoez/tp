package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedHighRound records review round 1 of spec.md holding one high
// finding, and returns the project and the recorded round's file.
func recordedHighRound(t *testing.T) (dir, roundFile string) {
	t.Helper()
	dir, spec := inboxSpec(t)
	for i, pm := range emittedPrompts(t, dir, noRoundDir(), "review", "spec.md") {
		body := ""
		if i == 0 {
			body = `{"severity":"high","finding":"a contradiction","location":"§1","evidence":"line 3"}` + "\n"
		}
		writeRoleFile(t, spec, pm["role"].(string), body)
	}
	_, stderr, code := runTPEnv(t, dir, noRoundDir(), "review", "spec.md", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	return dir, filepath.Join(".tp-review", "spec", "review-round-1.ndjson")
}

// resolveFixed resolves the round's only finding fixed under env and returns
// the payload.
func resolveFixed(t *testing.T, dir, roundFile string, env []string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTPEnv(t, dir, env, "review", roundFile, "--resolve", "0", "fixed", "repaired §1")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	return out
}

// TestReviewStatus_EveryBlockingFixedNamesOnlyTheNextRound: once every
// blocking finding of the latest round is resolved fixed, next_action names the
// round that verifies the repairs, not the resolve that already happened.
func TestReviewStatus_EveryBlockingFixedNamesOnlyTheNextRound(t *testing.T) {
	t.Parallel()
	dir, roundFile := recordedHighRound(t)
	resolveFixed(t, dir, roundFile, noRoundDir())

	stdout, stderr, code := runTPEnv(t, dir, noRoundDir(), "review", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	next := out["next_action"].(string)
	assert.Contains(t, next, "--record-inbox", "the next round verifies the repair")
	assert.NotContains(t, next, "--resolve <index> fixed", "the finding is already resolved fixed")
}

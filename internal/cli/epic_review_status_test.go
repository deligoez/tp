package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicStatusPayload is the part of tp epic review --status these tests read.
type epicStatusPayload struct {
	StateDir         string `json:"state_dir"`
	Round            int    `json:"round"`
	ConsecutiveClean int    `json:"consecutive_clean"`
	Converged        bool   `json:"converged"`
	Done             bool   `json:"done"`
	DoneBy           string `json:"done_by"`
	Stale            bool   `json:"stale"`
	InFlightRound    *int   `json:"in_flight_round"`
	NextAction       string `json:"next_action"`
}

// writeEpicProjectConfig writes .tp/config.json holding workflow.
func writeEpicProjectConfig(t *testing.T, dir, workflow string) {
	t.Helper()
	body := `{"workflow": ` + workflow + "}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"), []byte(body), 0o600))
}

// epicStatus runs tp epic review --status over the slice and decodes it.
func epicStatus(t *testing.T, dir string, slice ...string) epicStatusPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...), "--status")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicStatusPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// epicCheckCode runs tp epic review --status --check and returns its exit code.
func epicCheckCode(t *testing.T, dir string, slice ...string) int {
	t.Helper()
	_, _, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...), "--status", "--check")...)
	return code
}

// recordEpicRound records rows as the slice's next round through --record.
func recordEpicRound(t *testing.T, dir, rows string, slice ...string) {
	t.Helper()
	path := epicRecordFile(t, dir, rows)
	_, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...), "--record", path)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestEpicReviewStatus_TwoCleanRoundsConvergeUntilTheGraphChanges is §9 rows
// 9 and 19: with review_clean_rounds 2 at the project layer, --check exits 1
// after one clean round and 0 after the second; editing a's title then reads
// stale and not converged. Converging on one clean round, or reading
// convergence from the recorded rounds alone, fails it.
func TestEpicReviewStatus_TwoCleanRoundsConvergeUntilTheGraphChanges(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, `{"review_clean_rounds": 2}`)
	slice := []string{"--milestone", "A"}

	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)
	first := epicStatus(t, dir, slice...)
	assert.Equal(t, 1, first.Round)
	assert.Equal(t, 1, first.ConsecutiveClean)
	assert.False(t, first.Converged, "one clean round of two does not converge")
	assert.Equal(t, 1, epicCheckCode(t, dir, slice...))

	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)
	second := epicStatus(t, dir, slice...)
	assert.Equal(t, 2, second.ConsecutiveClean)
	assert.True(t, second.Converged)
	assert.True(t, second.Done)
	assert.Equal(t, "converged", second.DoneBy)
	assert.False(t, second.Stale)
	assert.Nil(t, second.InFlightRound)
	assert.Equal(t, 0, epicCheckCode(t, dir, slice...))

	editEpicGraph(t, dir, `"title": "A"`, `"title": "A renamed"`)
	edited := epicStatus(t, dir, slice...)
	assert.True(t, edited.Stale, "a's hash differs from the last recorded round's")
	assert.False(t, edited.Converged, "a stale slice is not converged")
	assert.False(t, edited.Done)
	assert.Equal(t, 1, epicCheckCode(t, dir, slice...))
}


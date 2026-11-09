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

// TestEpicReviewStatus_ACapWithAHighFindingRefusesUntilDispositioned is §9 row
// 10: with review_max_rounds 1, a recorded high finding reaches the cap, so
// emission and record exit 4 with the escalation hint and --check exits 1;
// once the finding is resolved wontfix with evidence, --check exits 0.
// Emitting round 2, or treating the cap alone as done, fails it.
func TestEpicReviewStatus_ACapWithAHighFindingRefusesUntilDispositioned(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, `{"review_max_rounds": 1}`)
	slice := []string{"--epic", "b"}
	row := `{"severity":"high","finding":"b's acceptance names no checker","location":"epic:b","evidence":"b"}`

	r1 := runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, row+"\n", slice...)

	_, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, slice...)...)
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "tp epic review --epic b --resolve", "the refusal names the slice's own resolve command")
	assert.Contains(t, stderr, "tp set --workflow --project review_max_rounds=", "the refusal names where the operator raises the cap")
	_, err := os.Stat(filepath.Join(dir, r1.StateDir, "snapshot-round-2.json"))
	assert.True(t, os.IsNotExist(err), "a refused emission opens no round 2")

	path := epicRecordFile(t, dir, "")
	_, stderr, code = runTP(t, dir, append(append([]string{"epic", "review"}, slice...), "--record", path)...)
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Len(t, epicRoundFiles(t, dir), 1, "a refused record writes no round")

	capped := epicStatus(t, dir, slice...)
	assert.False(t, capped.Done, "the cap alone is not done")
	assert.Equal(t, 1, epicCheckCode(t, dir, slice...))

	r1.Round = 1
	writeEpicRound(t, dir, &r1, `{"severity":"high","finding":"b's acceptance names no checker","location":"epic:b","evidence":"b","resolved":{"status":"wontfix","evidence":"the checker ships in a later milestone"}}`+"\n")
	resolved := epicStatus(t, dir, slice...)
	assert.True(t, resolved.Done)
	assert.Equal(t, "cap", resolved.DoneBy)
	assert.False(t, resolved.Converged)
	assert.Equal(t, 0, epicCheckCode(t, dir, slice...))
}

// epicChainWithCycle is row 11's fixture under D9: the chain plus a separate
// cycle x ↔ y that the slice --epic b does not reach.
const epicChainWithCycle = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "a needs b"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "c needs d"}]},
    {"id": "d", "title": "D", "milestone": "A"},
    {"id": "x", "title": "X", "milestone": "B", "depends_on": [{"id": "y", "why": "x needs y"}]},
    {"id": "y", "title": "Y", "milestone": "B", "depends_on": [{"id": "x", "why": "y needs x"}]}
  ]
}
`

// TestEpicReviewStatus_ALintErrorOutsideTheSliceKeepsCheckFailing is §9 row 11
// with D9: both clean rounds record and converge, but the cycle x ↔ y fails
// the cycle mechanical check, so --check stays at 1. Refusing emission on a
// lint error, or linting the slice only, fails it.
func TestEpicReviewStatus_ALintErrorOutsideTheSliceKeepsCheckFailing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithCycle)
	slice := []string{"--epic", "b"}

	for range 2 {
		out := runEpicReview(t, dir, slice...)
		failing := []string{}
		for _, c := range out.MechanicalChecks {
			if !c.Passed {
				failing = append(failing, c.Class)
			}
		}
		assert.Equal(t, []string{"cycle"}, failing)
		recordEpicRound(t, dir, "", slice...)
	}
	st := epicStatus(t, dir, slice...)
	assert.True(t, st.Converged, "the slice itself converged")
	assert.True(t, st.Done)
	assert.Contains(t, st.NextAction, "tp epic lint", "next_action names the failing checks' source")
	assert.Equal(t, 1, epicCheckCode(t, dir, slice...))
}

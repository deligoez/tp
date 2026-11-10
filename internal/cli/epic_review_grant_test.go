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

// epicGrantHigh is a high finding on b: it keeps a capped --epic b slice from
// being done at its cap, so only the graph's change is left to decide.
const epicGrantHigh = `{"severity":"high","finding":"b's acceptance names no checker","location":"epic:b","evidence":"b","resolved":{"status":"wontfix","evidence":"the checker ships later"}}` + "\n"

// epicGrantPayload is the part of tp epic review --grant-round's payload and
// of --status these tests read.
type epicGrantPayload struct {
	StateDir   string `json:"state_dir"`
	Round      int    `json:"round"`
	Grantable  *bool  `json:"grantable"`
	NextAction string `json:"next_action"`
}

// cappedStaleEpicSlice returns a project with review_max_rounds 1 whose
// --epic b slice recorded its one round and whose b changed after it.
func cappedStaleEpicSlice(t *testing.T) string {
	t.Helper()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, `{"review_max_rounds": 1}`)
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, epicGrantHigh, "--epic", "b")
	editEpicGraph(t, dir, `"title": "B"`, `"title": "B renamed"`)
	return dir
}

// epicGrantStatus runs tp epic review --epic b --status and decodes it.
func epicGrantStatus(t *testing.T, dir string) epicGrantPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicGrantPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// grantRound runs tp epic review --epic b --grant-round.
func grantRound(t *testing.T, dir string, env ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runTPEnv(t, dir, env, "epic", "review", "--epic", "b", "--grant-round")
}

// epicGrantFiles lists the grant files under dir's epic review state.
func epicGrantFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".tp", "epic-review", "*", "grant.json"))
	require.NoError(t, err)
	return files
}

// TestEpicReviewGrant_StatusMarksAStaleCappedSliceGrantable is spec/1.7.0.md
// §8's status half: a slice at its cap whose graph changed reads
// grantable: true, and its next_action names --grant-round before the project
// cap. A next_action that leads with review_max_rounds, or a grantable left
// false, fails it; an unchanged capped slice stays grantable: false.
func TestEpicReviewGrant_StatusMarksAStaleCappedSliceGrantable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, `{"review_max_rounds": 1}`)
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, `{"severity":"high","finding":"f","location":"epic:b","evidence":"b"}`+"\n", "--epic", "b")
	fresh := epicGrantStatus(t, dir)
	require.NotNil(t, fresh.Grantable, "--status always carries grantable")
	assert.False(t, *fresh.Grantable, "an unchanged slice at its cap has nothing for one more round to read")

	editEpicGraph(t, dir, `"title": "B"`, `"title": "B renamed"`)
	st := epicGrantStatus(t, dir)
	require.NotNil(t, st.Grantable)
	assert.True(t, *st.Grantable)
	grant := strings.Index(st.NextAction, "tp epic review --epic b --grant-round")
	require.GreaterOrEqual(t, grant, 0, "next_action names the grant: %s", st.NextAction)
	assert.Less(t, grant, strings.Index(st.NextAction, "review_max_rounds"), "the grant comes before the project cap")

	_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "--grant-round", "the cap refusal of a grantable slice names the grant")
}

// TestEpicReviewGrant_TheGrantedRoundEmitsRecordsAndIsSpent is §8's grant: on
// a stale slice at its cap --grant-round exits 0 naming round 2, round 2 emits
// with the ordinary carry (b asked again, nothing special in it) and records,
// and the recorded round spends the grant, so a third emission exits 4 again.
func TestEpicReviewGrant_TheGrantedRoundEmitsRecordsAndIsSpent(t *testing.T) {
	t.Parallel()
	dir := cappedStaleEpicSlice(t)

	stdout, stderr, code := grantRound(t, dir)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var granted epicGrantPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &granted), "stdout: %s", stdout)
	assert.Equal(t, 2, granted.Round)
	assert.Contains(t, granted.NextAction, "tp epic review --epic b")
	st := epicGrantStatus(t, dir)
	assert.False(t, *st.Grantable, "a granted slice is not grantable again")
	assert.Contains(t, st.NextAction, "emit round 2")

	r2 := runEpicReview(t, dir, "--epic", "b")
	assert.Equal(t, 2, r2.Round)
	assert.Equal(t, []string{"epic:b"}, r2.Units, "the granted round carries as any round does")
	recordEpicRound(t, dir, `{"severity":"high","finding":"f2","location":"epic:b","evidence":"b"}`+"\n", "--epic", "b")
	assert.Len(t, epicRoundFiles(t, dir), 2)

	_, stderr, code = runTP(t, dir, "epic", "review", "--epic", "b")
	require.Equal(t, 4, code, "the recorded round spent the grant; stderr: %s", stderr)
	assert.False(t, *epicGrantStatus(t, dir).Grantable, "the slice read its change and is not stale")
}

// TestEpicReviewGrant_RefusedUnlessAtTheCapAndStale is §8's refusal: below
// the cap, at the cap with the graph unchanged, and with a grant already open,
// --grant-round exits 4 — the cap refusal's code — and writes no grant.
func TestEpicReviewGrant_RefusedUnlessAtTheCapAndStale(t *testing.T) {
	t.Parallel()
	below := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, below, `{"review_max_rounds": 2}`)
	runEpicReview(t, below, "--epic", "b")
	recordEpicRound(t, below, epicGrantHigh, "--epic", "b")
	editEpicGraph(t, below, `"title": "B"`, `"title": "B renamed"`)
	_, stderr, code := grantRound(t, below)
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "review_max_rounds")
	assert.Empty(t, epicGrantFiles(t, below))

	unchanged := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, unchanged, `{"review_max_rounds": 1}`)
	runEpicReview(t, unchanged, "--epic", "b")
	recordEpicRound(t, unchanged, `{"severity":"high","finding":"f","location":"epic:b","evidence":"b"}`+"\n", "--epic", "b")
	_, stderr, code = grantRound(t, unchanged)
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "not changed")
	assert.Empty(t, epicGrantFiles(t, unchanged))

	open := cappedStaleEpicSlice(t)
	_, stderr, code = grantRound(t, open)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = grantRound(t, open)
	require.Equal(t, 4, code, "a second grant before the round records is refused; stderr: %s", stderr)
	assert.Contains(t, stderr, "already granted")
}

// TestEpicReviewGrant_UnattendedExitsTwoNamingGrantRound is §8 under tp run:
// the grant is the operator's, so under TP_UNATTENDED it exits 2 with a hint
// naming tp escalate --decision grant-round and writes no grant.
func TestEpicReviewGrant_UnattendedExitsTwoNamingGrantRound(t *testing.T) {
	t.Parallel()
	dir := cappedStaleEpicSlice(t)
	_, stderr, code := grantRound(t, dir, unattendedOn)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "tp escalate --decision grant-round")
	assert.Empty(t, epicGrantFiles(t, dir))
}


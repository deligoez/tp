package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicCapGrants is the part of a slice's --status, and of each batch row,
// that reports its cap and the grants it has spent (spec/1.8.0.md §5).
type epicCapGrants struct {
	Round       int  `json:"round"`
	MaxRounds   *int `json:"max_rounds"`
	GrantsSpent *int `json:"grants_spent"`
}

// slicePastItsCapWithOneGrant is §18 row 4's fixture: the capped stale
// --epic b slice (review_max_rounds 1) granted one round, which it emitted
// and recorded, so it stands at round 2 past its cap of 1.
func slicePastItsCapWithOneGrant(t *testing.T) string {
	t.Helper()
	dir := cappedStaleEpicSlice(t)
	_, stderr, code := grantRound(t, dir)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, `{"severity":"high","finding":"f2","location":"epic:b","evidence":"b"}`+"\n", "--epic", "b")
	return dir
}

// decodeCapGrants decodes one slice's --status.
func decodeCapGrants(t *testing.T, dir string, args ...string) epicCapGrants {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, args...), "--status")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicCapGrants
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// TestEpicReviewStatus_ReportsTheCapAndTheGrantsSpent is §5's cap and grants:
// a slice at round 2 past a cap of 1 with one grant reads max_rounds 1 and
// grants_spent 1, so "round 2, cap 1 + 1 grant" is readable; before the grant
// it read grants_spent 0. A grants_spent that stays 0, or one counted from the
// grant file instead of the rounds it bought, fails it.
func TestEpicReviewStatus_ReportsTheCapAndTheGrantsSpent(t *testing.T) {
	t.Parallel()
	before := decodeCapGrants(t, cappedStaleEpicSlice(t), "--epic", "b")
	require.NotNil(t, before.MaxRounds)
	require.NotNil(t, before.GrantsSpent, "--status always carries grants_spent")
	assert.Equal(t, 1, *before.MaxRounds)
	assert.Equal(t, 0, *before.GrantsSpent, "a slice at its cap has spent no grant")

	open := cappedStaleEpicSlice(t)
	_, stderr, code := grantRound(t, open)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	granted := decodeCapGrants(t, open, "--epic", "b")
	require.NotNil(t, granted.GrantsSpent)
	assert.Equal(t, 0, *granted.GrantsSpent, "a grant whose round is not recorded yet is given, not spent")

	st := decodeCapGrants(t, slicePastItsCapWithOneGrant(t), "--epic", "b")
	assert.Equal(t, 2, st.Round)
	require.NotNil(t, st.MaxRounds)
	require.NotNil(t, st.GrantsSpent)
	assert.Equal(t, 1, *st.MaxRounds)
	assert.Equal(t, 1, *st.GrantsSpent)
}

// TestEpicReviewBatchStatus_EachRowReportsTheCapAndTheGrantsSpent is the same
// under --all: the slice's row carries max_rounds and grants_spent as its own
// --status does.
func TestEpicReviewBatchStatus_EachRowReportsTheCapAndTheGrantsSpent(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runTP(t, slicePastItsCapWithOneGrant(t), "epic", "review", "--all", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Slices []epicCapGrants `json:"slices"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.Len(t, out.Slices, 1)
	row := out.Slices[0]
	assert.Equal(t, 2, row.Round)
	require.NotNil(t, row.MaxRounds)
	require.NotNil(t, row.GrantsSpent)
	assert.Equal(t, 1, *row.MaxRounds)
	assert.Equal(t, 1, *row.GrantsSpent)
}

// TestEpicReviewStatus_AnUncappedSliceReportsANullCap: with review_max_rounds
// 0 the slice has no cap, so max_rounds is null and no grant was spent.
func TestEpicReviewStatus_AnUncappedSliceReportsANullCap(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, `{"review_max_rounds": 0}`)
	st := decodeCapGrants(t, dir, "--epic", "b")
	assert.Nil(t, st.MaxRounds)
	require.NotNil(t, st.GrantsSpent)
	assert.Equal(t, 0, *st.GrantsSpent)
}

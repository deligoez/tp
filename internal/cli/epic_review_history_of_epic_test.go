package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicOfEpicPayload is tp epic review --history --of-epic <id>'s payload as
// these tests read it.
type epicOfEpicPayload struct {
	Epic   string `json:"epic"`
	Slices []struct {
		StateDir string `json:"state_dir"`
		Key      *struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		} `json:"key"`
		Rounds []struct {
			Round int `json:"round"`
			Edges []struct {
				To  string `json:"to"`
				Why string `json:"why"`
			} `json:"edges"`
			HashesOnly bool `json:"hashes_only"`
		} `json:"rounds"`
	} `json:"slices"`
	Message string `json:"message"`
}

// epicOfEpic runs tp epic review --history --of-epic id and decodes it.
func epicOfEpic(t *testing.T, dir, id string) epicOfEpicPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "review", "--history", "--of-epic", id)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicOfEpicPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// TestEpicReviewHistoryOfEpic_ListsEdgesPerRoundOfEverySlice is §11's
// --of-epic: b's edges and whys per recorded round of every slice whose
// snapshot holds it — the milestone A slice, whose two rounds lie either side
// of the rewrite of b -> c's why, lists that edge under both rounds with each
// text, and the --epic b slice recorded after it lists it once.
func TestEpicReviewHistoryOfEpic_ListsEdgesPerRoundOfEverySlice(t *testing.T) {
	t.Parallel()
	dir, _ := twoRoundSlice(t)
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, "", "--epic", "b")

	h := epicOfEpic(t, dir, "b")
	assert.Equal(t, "b", h.Epic)
	assert.Empty(t, h.Message)
	require.Len(t, h.Slices, 2)
	byKind := map[string]int{}
	for i, s := range h.Slices {
		require.NotNil(t, s.Key)
		byKind[s.Key.Kind] = i
		assert.NotEmpty(t, s.StateDir)
	}

	m := h.Slices[byKind["milestone"]]
	assert.Equal(t, "A", m.Key.Value)
	require.Len(t, m.Rounds, 2)
	for i, why := range []string{"b needs c", "b reads c's schema"} {
		assert.Equal(t, i+1, m.Rounds[i].Round)
		require.Len(t, m.Rounds[i].Edges, 1)
		assert.Equal(t, "c", m.Rounds[i].Edges[0].To)
		assert.Equal(t, why, m.Rounds[i].Edges[0].Why)
	}

	e := h.Slices[byKind["epic"]]
	require.Len(t, e.Rounds, 1)
	require.Len(t, e.Rounds[0].Edges, 1)
	assert.Equal(t, "b reads c's schema", e.Rounds[0].Edges[0].Why)
}

// TestEpicReviewHistoryOfEpic_AnEpicInNoRecordedSliceSaysSo is §11's
// acceptance: an epic no recorded round's snapshot holds — here one whose
// only slice was emitted and never recorded — reports no history and says so.
func TestEpicReviewHistoryOfEpic_AnEpicInNoRecordedSliceSaysSo(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--epic", "a")

	h := epicOfEpic(t, dir, "a")
	assert.Equal(t, "a", h.Epic)
	assert.NotNil(t, h.Slices)
	assert.Empty(t, h.Slices)
	assert.Contains(t, h.Message, "no recorded")
}


package cli_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sliceReviewFields is the part of a slice's --status that tp epic export's
// slices[] entry carries too (spec/1.8.0.md §5).
type sliceReviewFields struct {
	MaxRounds   *int            `json:"max_rounds"`
	GrantsSpent *int            `json:"grants_spent"`
	StaleUnits  *[]string       `json:"stale_units"`
	OpenByRole  *map[string]int `json:"open_by_role"`
}

// TestEpicExport_EachSliceCarriesItsStatusReviewFields is §5: the --epic b
// slice, at round 2 past its cap of 1 with one grant, its second round's
// finding naming no role, and c's title edited after it (b's unit hashes c's
// block), reads the same max_rounds, grants_spent, stale_units and
// open_by_role in the export as in its --status; the entry holds no other
// key, so no per-round history enters the export. An export that leaves one
// of the four out, or adds the rounds, turns it red.
func TestEpicExport_EachSliceCarriesItsStatusReviewFields(t *testing.T) {
	t.Parallel()
	dir := slicePastItsCapWithOneGrant(t)
	editEpicGraph(t, dir, `"title": "C"`, `"title": "C renamed"`)

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var status sliceReviewFields
	require.NoError(t, json.Unmarshal([]byte(stdout), &status), "stdout: %s", stdout)
	require.NotNil(t, status.StaleUnits)
	require.Equal(t, []string{"epic:b"}, *status.StaleUnits, "the fixture is stale through b")

	stdout, stderr, code = runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var export struct {
		Slices []json.RawMessage `json:"slices"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &export), "stdout: %s", stdout)
	require.Len(t, export.Slices, 1)
	var entry sliceReviewFields
	require.NoError(t, json.Unmarshal(export.Slices[0], &entry))
	assert.Equal(t, status, entry, "the export's slice carries what --status reports")
	for name, v := range map[string]any{"max_rounds": entry.MaxRounds, "grants_spent": entry.GrantsSpent,
		"stale_units": entry.StaleUnits, "open_by_role": entry.OpenByRole} {
		assert.NotNil(t, v, "slices[] carries %s", name)
	}

	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(export.Slices[0], &keys))
	got := make([]string, 0, len(keys))
	for k := range keys {
		got = append(got, k)
	}
	slices.Sort(got)
	assert.Equal(t, []string{"dir", "grantable", "grants_spent", "key", "max_rounds", "open_by_role", "round", "stale_units", "state"}, got,
		"no per-round history enters the export")
}

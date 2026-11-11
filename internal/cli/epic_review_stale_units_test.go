package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicStaleUnits is the part of a slice's --status, and of each batch row,
// that names the units changed since its last recorded round
// (spec/1.8.0.md §5).
type epicStaleUnits struct {
	Stale      bool      `json:"stale"`
	StaleUnits *[]string `json:"stale_units"`
}

// decodeStaleUnits runs tp epic review <slice> --status and decodes it.
func decodeStaleUnits(t *testing.T, dir string, slice ...string) epicStaleUnits {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...), "--status")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicStaleUnits
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// TestEpicReviewStatus_AStaleSliceNamesTheUnitsThatChanged is §18 row 4: the
// milestone A slice, fresh after its recorded round, lists no stale unit;
// once a's title is edited it is stale and names exactly epic:a, not the
// milestone's other units (a is the epic no unit's hash covers: b's edit
// would change a's unit too, since a unit hashes its dependencies' blocks).
// The mutant reading staleness from the milestone
// instead of the slice's units, naming every unit of A, turns it red.
func TestEpicReviewStatus_AStaleSliceNamesTheUnitsThatChanged(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)

	fresh := decodeStaleUnits(t, dir, slice...)
	assert.False(t, fresh.Stale)
	require.NotNil(t, fresh.StaleUnits, "--status always carries stale_units")
	assert.Equal(t, []string{}, *fresh.StaleUnits, "a fresh slice lists none")

	editEpicGraph(t, dir, `"title": "A"`, `"title": "A renamed"`)
	stale := decodeStaleUnits(t, dir, slice...)
	assert.True(t, stale.Stale)
	require.NotNil(t, stale.StaleUnits)
	assert.Equal(t, []string{"epic:a"}, *stale.StaleUnits)

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--all", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var batch struct {
		Slices []epicStaleUnits `json:"slices"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &batch), "stdout: %s", stdout)
	require.Len(t, batch.Slices, 1)
	require.NotNil(t, batch.Slices[0].StaleUnits, "each batch row carries stale_units")
	assert.Equal(t, []string{"epic:a"}, *batch.Slices[0].StaleUnits)
}

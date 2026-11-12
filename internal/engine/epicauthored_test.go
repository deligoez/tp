package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// carryAfter records round 1 of key's slice over before, finding nothing,
// and returns round 2's carry over after.
func carryAfter(t *testing.T, before, after *model.EpicGraph, key EpicSliceKey) EpicCarry {
	t.Helper()
	dir := t.TempDir()
	first, err := EpicReviewSlice(before, key)
	require.NoError(t, err)
	require.NoError(t, WriteEpicReviewSnapshot(dir, 1, first.Units, nil))
	require.NoError(t, os.WriteFile(filepath.Join(dir, epicRoundFile(1)), nil, 0o600))
	second, err := EpicReviewSlice(after, key)
	require.NoError(t, err)
	carry, err := EpicReviewCarry(dir, 2, second.Units)
	require.NoError(t, err)
	return carry
}

// carryAfterLegacy is carryAfter over a round 1 snapshot as v1.10.0 wrote
// it: hashes and blocks, no own_rests and no tails.
func carryAfterLegacy(t *testing.T, before, after *model.EpicGraph, key EpicSliceKey) EpicCarry {
	t.Helper()
	dir := t.TempDir()
	first, err := EpicReviewSlice(before, key)
	require.NoError(t, err)
	require.NoError(t, WriteEpicReviewSnapshot(dir, 1, first.Units, nil))
	path := filepath.Join(dir, epicSnapshotFile(1))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var snap map[string]any
	require.NoError(t, json.Unmarshal(data, &snap))
	require.Contains(t, snap, "own_rests")
	delete(snap, "own_rests")
	delete(snap, "tails")
	data, err = json.Marshal(snap)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, epicRoundFile(1)), nil, 0o600))
	second, err := EpicReviewSlice(after, key)
	require.NoError(t, err)
	carry, err := EpicReviewCarry(dir, 2, second.Units)
	require.NoError(t, err)
	return carry
}

// TestEpicCarry_OverAV1100SnapshotADescriptionAloneIsAuthored is the upgrade
// path: a snapshot from before v1.11.0 holds no own_rests or tails, and could
// hold no description, so a block that only gained one is Authored and
// appends no regression.
func TestEpicCarry_OverAV1100SnapshotADescriptionAloneIsAuthored(t *testing.T) {
	before := epicDescribedGraph()
	after := epicDescribedGraph()
	after.Epics[0].Description = "Purpose: read b."
	carry := carryAfterLegacy(t, before, after, EpicSliceKey{Epic: "a"})
	assert.Equal(t, map[string]bool{"epic:a": true}, carry.Authored)
	assert.False(t, carry.Regression())
}

// TestEpicCarry_OverAV1100SnapshotADescriptionWithAnAcceptanceEditIsAChange:
// over the same snapshot, a description added beside an acceptance edit is
// a change: not Authored, and regression runs.
func TestEpicCarry_OverAV1100SnapshotADescriptionWithAnAcceptanceEditIsAChange(t *testing.T) {
	before := epicDescribedGraph()
	after := epicDescribedGraph()
	after.Epics[0].Description = "Purpose: read b."
	after.Epics[0].Acceptance = "a ships and reads b"
	carry := carryAfterLegacy(t, before, after, EpicSliceKey{Epic: "a"})
	assert.Empty(t, carry.Authored)
	assert.True(t, carry.Regression())
}

// rootEpicGraph is one root epic b with acceptance acc and no description.
func rootEpicGraph(acc string) *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}},
		Epics: []model.Epic{{ID: "b", Title: "B step", Milestone: "A", Root: true, Acceptance: acc}}}
}

// TestEpicCarry_AnAcceptanceShapedLikeADescriptionIsAChange: an acceptance
// edit whose text holds a description: line and indented lines is a
// contract change, whatever its shape: it is not Authored and appends
// regression.
func TestEpicCarry_AnAcceptanceShapedLikeADescriptionIsAChange(t *testing.T) {
	carry := carryAfter(t, rootEpicGraph("b ships\ndescription:\n  step one"),
		rootEpicGraph("b ships\ndescription:\n  step two, much bigger"), EpicSliceKey{Epic: "b"})
	assert.Empty(t, carry.Authored)
	assert.True(t, carry.Regression())
}

// TestEpicCarry_CuttingADescriptionShapedAcceptanceIsAChange is the mirror:
// an acceptance holding a description: line and indented lines, edited
// down to its first line, is a change too.
func TestEpicCarry_CuttingADescriptionShapedAcceptanceIsAChange(t *testing.T) {
	carry := carryAfter(t, rootEpicGraph("foo\ndescription:\n  bar\n  baz"), rootEpicGraph("foo"), EpicSliceKey{Epic: "b"})
	assert.Empty(t, carry.Authored)
	assert.True(t, carry.Regression())
}

// TestEpicCarry_ADescriptionBesideADependencyEditIsAChange: on the --epic a
// slice, a's description and its dependency b's acceptance change in one
// round; a's hash moved for b too, so a is not Authored and regression runs.
func TestEpicCarry_ADescriptionBesideADependencyEditIsAChange(t *testing.T) {
	before := epicDescribedGraph()
	after := epicDescribedGraph()
	after.Epics[0].Description = "Purpose: read b."
	after.Epics[1].Acceptance = "b ships twice"
	carry := carryAfter(t, before, after, EpicSliceKey{Epic: "a"})
	assert.Empty(t, carry.Authored)
	assert.True(t, carry.Regression())
}

// TestEpicCarry_ADescriptionAloneIsAuthored: the positive case, a's
// description alone changed, is Authored and appends no regression.
func TestEpicCarry_ADescriptionAloneIsAuthored(t *testing.T) {
	before := epicDescribedGraph()
	after := epicDescribedGraph()
	after.Epics[0].Description = "Purpose: read b."
	carry := carryAfter(t, before, after, EpicSliceKey{Epic: "a"})
	assert.Equal(t, map[string]bool{"epic:a": true}, carry.Authored)
	assert.False(t, carry.Regression())
}

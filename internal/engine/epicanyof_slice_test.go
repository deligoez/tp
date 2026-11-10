package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// anyOfSliceGraph is milestone A holding the roots a and b and w, which
// depends on one of {a, b}.
func anyOfSliceGraph() *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}, Epics: []model.Epic{
		{ID: "a", Title: "A step", Milestone: "A", Root: true},
		{ID: "b", Title: "B step", Milestone: "A", Root: true},
		{ID: "w", Title: "W step", Milestone: "A", DependsOn: []model.EpicDep{{AnyOf: []string{"a", "b"}, Why: "either opens the API"}}},
	}}
}

// TestEpicReviewSlice_AnEpicBlockRendersAGroupLine is spec/1.7.0.md §7: w's
// block lists its group as one of {a, b} with its why. A block that read a
// group's id renders "- : either opens the API".
func TestEpicReviewSlice_AnEpicBlockRendersAGroupLine(t *testing.T) {
	s, err := EpicReviewSlice(anyOfSliceGraph(), EpicSliceKey{Epic: "w"})
	require.NoError(t, err)
	require.Len(t, s.Units, 1)
	assert.Contains(t, s.Units[0].Block, "depends_on:\n- one of {a, b}: either opens the API\n")
}

// TestEpicReviewSlice_AUnitHashCoversItsGroupMembersBlocks is spec/1.7.0.md
// §7: a group's members are w's dependencies, so editing b's title re-asks
// w as editing a plain dependency does. A hash that read a group's id
// covers no member's block.
func TestEpicReviewSlice_AUnitHashCoversItsGroupMembersBlocks(t *testing.T) {
	before := unitHashes(t, anyOfSliceGraph())
	g := anyOfSliceGraph()
	g.Epics[1].Title = "B step, renamed"
	after := unitHashes(t, g)
	assert.NotEqual(t, before["epic:w"], after["epic:w"])
	assert.Equal(t, before["epic:a"], after["epic:a"])
}

// contextOf returns the context of epic id's slice of g by id.
func contextOf(t *testing.T, g *model.EpicGraph, id string) map[string]EpicContextBlock {
	t.Helper()
	s, err := EpicReviewSlice(g, EpicSliceKey{Epic: id})
	require.NoError(t, err)
	out := map[string]EpicContextBlock{}
	for _, c := range s.Context {
		out[c.ID] = c
	}
	return out
}

// TestEpicReviewSlice_AGroupsMembersAreDependencyContext is spec/1.7.0.md
// §7: asking w shows both members of its group as dependencies. A context
// that read a group's id shows neither.
func TestEpicReviewSlice_AGroupsMembersAreDependencyContext(t *testing.T) {
	ctx := contextOf(t, anyOfSliceGraph(), "w")
	require.Contains(t, ctx, "a")
	require.Contains(t, ctx, "b")
	assert.True(t, ctx["a"].Dependency)
	assert.True(t, ctx["b"].Dependency)
}

// TestEpicReviewSlice_AnEpicWithAGroupIntoAnAskedOneIsADependent is
// spec/1.7.0.md §7: asking a shows w, whose group names a, as a dependent,
// and its neighbours-brief shows that group with its why. A brief that
// matched a group's id shows no edge into a.
func TestEpicReviewSlice_AnEpicWithAGroupIntoAnAskedOneIsADependent(t *testing.T) {
	ctx := contextOf(t, anyOfSliceGraph(), "a")
	require.Contains(t, ctx, "w")
	assert.False(t, ctx["w"].Dependency)
	assert.Contains(t, ctx["w"].Brief, "depends_on (its edges into the asked units only):\n- one of {a, b}: either opens the API\n")
}

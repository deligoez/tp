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

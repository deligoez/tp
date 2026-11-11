package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// TestMilestoneMatrix_CountsEdgesBetweenMilestones is §4's milestone matrix:
// one cell per pair of milestones an edge joins, from the dependent epic's
// milestone to its dependency's, counting every edge, each member of an any-of
// group one, and how many of their dependencies are done; an edge inside one
// milestone has no cell.
func TestMilestoneMatrix_CountsEdgesBetweenMilestones(t *testing.T) {
	t.Parallel()
	g := &model.EpicGraph{
		Milestones: [][]string{{"M1", "M2"}},
		Epics: []model.Epic{
			{ID: "a", Milestone: "M1"},
			{ID: "b", Milestone: "M1"},
			{ID: "c", Milestone: "M2", DependsOn: []model.EpicDep{{ID: "a", Why: "w"}, {ID: "b", Why: "w"}}},
			{ID: "d", Milestone: "M2", DependsOn: []model.EpicDep{{AnyOf: []string{"a", "b"}, Why: "w"}}},
			{ID: "e", Milestone: "M2", DependsOn: []model.EpicDep{{ID: "c", Why: "w"}}},
		},
	}
	states := map[string]EpicState{"a": {Status: "done"}, "b": {Status: "planned"}, "c": {Status: "planned"}, "d": {Status: "planned"}, "e": {Status: "planned"}}
	assert.Equal(t, []MatrixCell{{From: "M2", To: "M1", Edges: 4, Done: 2}}, MilestoneMatrix(g, states))
	assert.Equal(t, []MatrixCell{}, MilestoneMatrix(&model.EpicGraph{}, nil), "a graph with no edge has no cell, as a list")
}

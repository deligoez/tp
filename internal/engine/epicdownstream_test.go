package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// cycleDownstreamGraph is milestone P holding r, a depending on r and b, and
// b depending on a, in the given order of ids: the cycle a <-> b sits
// downstream of r.
func cycleDownstreamGraph(order ...string) *model.EpicGraph {
	all := map[string]model.Epic{
		"r": {ID: "r", Title: "R", Milestone: "P", Root: true},
		"a": {ID: "a", Title: "A", Milestone: "P", DependsOn: []model.EpicDep{{ID: "r", Why: "r first"}, {ID: "b", Why: "b first"}}},
		"b": {ID: "b", Title: "B", Milestone: "P", DependsOn: []model.EpicDep{{ID: "a", Why: "a first"}}},
	}
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}}
	for _, id := range order {
		g.Epics = append(g.Epics, all[id])
	}
	return g
}

// TestEpicDownstreams_CycleMembersCountEachOtherWhateverTheOrder: on a
// dependency cycle every member reaches the others, so a and b each count
// the other, and r both, in every order of the graph's epics.
func TestEpicDownstreams_CycleMembersCountEachOtherWhateverTheOrder(t *testing.T) {
	for _, order := range [][]string{{"r", "a", "b"}, {"r", "b", "a"}, {"b", "a", "r"}} {
		got := EpicDownstreams(cycleDownstreamGraph(order...), map[string]EpicState{}, map[string]MilestoneGate{}, []string{"r", "a", "b"})
		for id, want := range map[string]int{"r": 2, "a": 1, "b": 1} {
			assert.Equal(t, want, got[id].Dependents, "%s dependents, order %v", id, order)
			assert.Equal(t, want, got[id].LongestChain, "%s longest_chain, order %v", id, order)
		}
	}
}

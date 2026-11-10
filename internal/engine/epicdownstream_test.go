package engine

import (
	"fmt"
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

// TestEpicDownstreams_CountsWorkPerMilestoneAndPassesConditionsThrough is
// spec/1.6.0.md F1 on track [P, Q, R, S]: r has the work dependent a in P,
// the chain k -> b -> c through the condition k in Q, and the cycle e <-> m
// in S, m a condition. Only a, b, c and e count; the chain through k is two
// long; Q, which holds only the condition, is not listed.
func TestEpicDownstreams_CountsWorkPerMilestoneAndPassesConditionsThrough(t *testing.T) {
	cond := func(id, m string, deps ...string) model.Epic {
		e := dependsOn(id, m, deps...)
		e.Condition, e.Acceptance = true, "the operator decides"
		return e
	}
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P", "Q", "R", "S"}}, Epics: []model.Epic{
		dependsOn("r", "P"), dependsOn("a", "P", "r"), cond("k", "Q", "r"), dependsOn("b", "R", "k"),
		dependsOn("c", "R", "b"), dependsOn("e", "S", "r", "m"), cond("m", "S", "e"),
	}}
	got := EpicDownstreams(g, map[string]EpicState{}, map[string]MilestoneGate{"P": {Open: true}}, []string{"r"})
	assert.Equal(t, EpicDownstream{LongestChain: 2, Dependents: 4, DependentsOpen: 1, DependentsByMilestone: []MilestoneDependents{
		{Milestone: "P", Dependents: 1}, {Milestone: "R", Dependents: 2}, {Milestone: "S", Dependents: 1},
	}}, got["r"])
}

// TestEpicDownstreams_ReadsEpicsPastTheFirst64: on a graph of 65 epics the
// cycle a <-> b sits at positions 63 and 64, across the first word of the
// walk's position sets, downstream of r at position 0. r counts both, each
// counts the other, each in its own milestone.
func TestEpicDownstreams_ReadsEpicsPastTheFirst64(t *testing.T) {
	epics := []model.Epic{dependsOn("r", "P")}
	for i := 1; i < 63; i++ {
		epics = append(epics, dependsOn(fmt.Sprintf("f%02d", i), "P"))
	}
	epics = append(epics, dependsOn("a", "Q", "r", "b"), dependsOn("b", "R", "a"))
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P", "Q", "R"}}, Epics: epics}
	got := EpicDownstreams(g, map[string]EpicState{}, map[string]MilestoneGate{}, []string{"r", "a", "b"})
	in := func(m string) []MilestoneDependents { return []MilestoneDependents{{Milestone: m, Dependents: 1}} }
	assert.Equal(t, EpicDownstream{LongestChain: 2, Dependents: 2, DependentsByMilestone: append(in("Q"), in("R")...)}, got["r"])
	assert.Equal(t, EpicDownstream{LongestChain: 1, Dependents: 1, DependentsByMilestone: in("R")}, got["a"])
	assert.Equal(t, EpicDownstream{LongestChain: 1, Dependents: 1, DependentsByMilestone: in("Q")}, got["b"])
}

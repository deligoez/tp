package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// withAnyOf returns e depending also on the any-of group of members.
func withAnyOf(e model.Epic, members ...string) model.Epic {
	e.Root = false
	e.DependsOn = append(e.DependsOn, model.EpicDep{AnyOf: members, Why: e.ID + " needs one of them"})
	return e
}

// TestEpicDownstreams_AGroupCountsInEachMembersDownstreamUntilSatisfied is
// spec/1.7.0.md §7: w depends on one of {a, b}, and v on w and on one of
// {a, c}. While no member is done, a reaches w and v, v once though two
// paths lead to it, and b reaches w and v; once b is done the group {a, b}
// leaves a's downstream, which keeps v through {a, c}. A walk that read a
// group as an edge to "" counts nothing; one that summed paths counts v
// twice.
func TestEpicDownstreams_AGroupCountsInEachMembersDownstreamUntilSatisfied(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("a", "P"), dependsOn("b", "P"), dependsOn("c", "P"),
		withAnyOf(dependsOn("w", "P"), "a", "b"),
		withAnyOf(dependsOn("v", "P", "w"), "a", "c"),
	}}
	in := func(n int) []MilestoneDependents { return []MilestoneDependents{{Milestone: "P", Dependents: n}} }
	got := EpicDownstreams(g, anyOfStates(g), map[string]MilestoneGate{}, []string{"a", "b", "c"})
	assert.Equal(t, EpicDownstream{LongestChain: 2, Dependents: 2, DependentsByMilestone: in(2)}, got["a"])
	assert.Equal(t, EpicDownstream{LongestChain: 2, Dependents: 2, DependentsByMilestone: in(2)}, got["b"])
	assert.Equal(t, EpicDownstream{LongestChain: 1, Dependents: 1, DependentsByMilestone: in(1)}, got["c"])

	got = EpicDownstreams(g, anyOfStates(g, "b"), map[string]MilestoneGate{}, []string{"a", "b"})
	assert.Equal(t, EpicDownstream{LongestChain: 1, Dependents: 1, DependentsByMilestone: in(1)}, got["a"])
	assert.NotContains(t, got, "b")
}

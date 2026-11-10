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

// TestEpicDownstreams_AMootEpicLeavesEveryCount is spec/1.7.0.md §7: w
// depends on one of {a, b}, a is done and b, which depends on r, lost; no
// gate holds b, so it is moot. It leaves r's downstream and has none of its
// own. A walk that judged only done counts b in r's dependents.
func TestEpicDownstreams_AMootEpicLeavesEveryCount(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("r", "P"), dependsOn("a", "P"), dependsOn("b", "P", "r"),
		withAnyOf(dependsOn("w", "P"), "a", "b"),
	}}
	g.Gates = map[string]model.EpicGate{"P": {Members: []string{"r", "a", "w"}}}
	got := EpicDownstreams(g, anyOfStates(g, "a"), map[string]MilestoneGate{}, []string{"r", "b"})
	assert.Equal(t, EpicDownstream{DependentsByMilestone: []MilestoneDependents{}}, got["r"])
	assert.NotContains(t, got, "b")
}

// TestGraphCriticalPath_GoesThroughTheGroupMemberWithTheShortestChain is
// spec/1.7.0.md §7: v depends on w, and w on one of {a, b2}, b2 on b. a's
// chain is one epic and b2's two, so the path takes a, though the longer
// chain through b2 is heavier; on a tie, as for y on one of {c, d}, the
// first listed member. A walk that read a group as an edge to "" never
// joins w to a member; one that maximised over members goes through b2.
func TestGraphCriticalPath_GoesThroughTheGroupMemberWithTheShortestChain(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("b", "P"), dependsOn("b2", "P", "b"), dependsOn("a", "P"),
		withAnyOf(dependsOn("w", "P"), "a", "b2"), dependsOn("v", "P", "w"),
	}}
	assert.Equal(t, []string{"a", "w", "v"}, GraphCriticalPath(g, anyOfStates(g)))

	g = &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("c", "P"), dependsOn("d", "P"), withAnyOf(dependsOn("y", "P"), "d", "c"),
	}}
	assert.Equal(t, []string{"d", "y"}, GraphCriticalPath(g, anyOfStates(g)))
}

// TestGraphCriticalPath_LeavesAMootEpicOut is spec/1.7.0.md §7: b, which
// depends on r, lost w's group to the done a and no gate holds it, so it is
// moot and the path is r alone. A path that judged only done runs r -> b.
func TestGraphCriticalPath_LeavesAMootEpicOut(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("r", "P"), dependsOn("a", "P"), dependsOn("b", "P", "r"),
		withAnyOf(dependsOn("w", "P"), "a", "b"),
	}}
	g.Gates = map[string]model.EpicGate{"P": {Members: []string{"r", "a", "w"}}}
	assert.Equal(t, []string{"r"}, GraphCriticalPath(g, anyOfStates(g, "a")))
}

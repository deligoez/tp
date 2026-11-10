package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// anyOfEpic is epic id of milestone m depending on the epics of plain and on
// the any-of group of members.
func anyOfEpic(members []string, id, m string, plain ...string) model.Epic {
	e := dependsOn(id, m, plain...)
	e.Root = false
	e.DependsOn = append(e.DependsOn, model.EpicDep{AnyOf: members, Why: id + " needs one of them"})
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
		anyOfEpic([]string{"a", "b"}, "w", "P"),
		anyOfEpic([]string{"a", "c"}, "v", "P", "w"),
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
		anyOfEpic([]string{"a", "b"}, "w", "P"),
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
		anyOfEpic([]string{"a", "b2"}, "w", "P"), dependsOn("v", "P", "w"),
	}}
	assert.Equal(t, []string{"a", "w", "v"}, GraphCriticalPath(g, anyOfStates(g)))

	g = &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("c", "P"), dependsOn("d", "P"), anyOfEpic([]string{"d", "c"}, "y", "P"),
	}}
	assert.Equal(t, []string{"d", "y"}, GraphCriticalPath(g, anyOfStates(g)))

	g = &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("s", "P"), dependsOn("l2", "P"), dependsOn("l", "P", "l2"),
		anyOfEpic([]string{"s", "l"}, "w", "P"), dependsOn("v", "P", "w"),
	}}
	assert.Equal(t, []string{"s", "w", "v"}, GraphCriticalPath(g, anyOfStates(g)),
		"the shortest member is kept when it is the graph's first epic")
}

// TestGraphCriticalPath_LeavesAMootEpicOut is spec/1.7.0.md §7: b, which
// depends on r, lost w's group to the done a and no gate holds it, so it is
// moot and the path is r alone. A path that judged only done runs r -> b.
func TestGraphCriticalPath_LeavesAMootEpicOut(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"P"}}, Epics: []model.Epic{
		dependsOn("r", "P"), dependsOn("a", "P"), dependsOn("b", "P", "r"),
		anyOfEpic([]string{"a", "b"}, "w", "P"),
	}}
	g.Gates = map[string]model.EpicGate{"P": {Members: []string{"r", "a", "w"}}}
	assert.Equal(t, []string{"r"}, GraphCriticalPath(g, anyOfStates(g, "a")))
}

// TestCriticalPath_GoesThroughTheGroupMemberWithTheShortestChain is
// spec/1.7.0.md §7: m, A's gate, depends on one of {x, y}, x on x2. By count
// the path takes y, one epic against x's two; by estimate it takes x, 20
// minutes against y's 50, the lightest. A walk that read a group as an edge
// to "" stops at m; one that maximised over members goes through x by count
// and y by estimate.
func TestCriticalPath_GoesThroughTheGroupMemberWithTheShortestChain(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}, {"B"}}, Epics: []model.Epic{
		anyOfEpic([]string{"x", "y"}, "m", "A"), dependsOn("x", "A", "x2"), dependsOn("x2", "B"), dependsOn("y", "B"),
	}, Gates: map[string]model.EpicGate{"A": {Members: []string{"m"}}}}
	path := CriticalPath(g, anyOfStates(g), map[string]int{"m": 5, "x": 10, "x2": 10, "y": 50}, "A")
	assert.Equal(t, EpicCriticalPath{ByCount: []string{"y", "m"}, ByEstimate: []string{"x2", "x", "m"}, EstimateMinutes: 25}, path)
}

// TestCriticalPath_TakesTheFirstListedMemberOnATie: m depends on one of
// {y, x}, each one epic of 10 minutes, so both chains go through y, the
// first listed. A walk that kept the last tied member goes through x.
func TestCriticalPath_TakesTheFirstListedMemberOnATie(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}, Epics: []model.Epic{
		anyOfEpic([]string{"y", "x"}, "m", "A"), dependsOn("x", "A"), dependsOn("y", "A"),
	}, Gates: map[string]model.EpicGate{"A": {Members: []string{"m"}}}}
	path := CriticalPath(g, anyOfStates(g), map[string]int{"m": 5, "x": 10, "y": 10}, "A")
	assert.Equal(t, EpicCriticalPath{ByCount: []string{"y", "m"}, ByEstimate: []string{"y", "m"}, EstimateMinutes: 15}, path)
}

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// opensLaterGraph is tracks [[A], [B]] with B opening after A, holding epics.
func opensLaterGraph(epics ...model.Epic) *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}, {"B"}},
		OpensAfter: map[string][]string{"B": {"A"}},
		Epics:      epics,
	}
}

// dependsOn is an epic of milestone m depending on ids, root when it has none.
func dependsOn(id, m string, ids ...string) model.Epic {
	e := model.Epic{ID: id, Title: id, Milestone: m, Root: len(ids) == 0}
	for _, d := range ids {
		e.DependsOn = append(e.DependsOn, model.EpicDep{ID: d, Why: id + " needs " + d})
	}
	return e
}

// TestLintEpicGraph_DependencyOpensLaterMemberIsAnError is fixture (i): a,
// a member of A's gate, depends on b in B, and B opens after A, so A's gate
// can never go green before B opens: one error naming a and b.
func TestLintEpicGraph_DependencyOpensLaterMemberIsAnError(t *testing.T) {
	g := opensLaterGraph(dependsOn("a", "A", "b"), dependsOn("b", "B"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityError, found[0].Severity)
	assert.Equal(t, []string{"a", "b"}, found[0].Epics)
	assert.Equal(t, []string{"A", "B"}, found[0].Names)
	assert.Contains(t, found[0].Message, "a -> b")
}

// TestLintEpicGraph_DependencyOpensLaterThroughADeferrableIsAnError is
// fixture (ii): the member a reaches b through c, a deferrable epic of A, so
// A's gate still waits on B: one error with the path, and no warning for c.
func TestLintEpicGraph_DependencyOpensLaterThroughADeferrableIsAnError(t *testing.T) {
	g := opensLaterGraph(dependsOn("a", "A", "c"), dependsOn("c", "A", "b"), dependsOn("b", "B"))
	g.Gates = map[string]model.EpicGate{"A": {Deferrable: []string{"c"}}}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityError, found[0].Severity)
	assert.Equal(t, []string{"a", "b"}, found[0].Epics)
	assert.Contains(t, found[0].Message, "a -> c -> b")
}

// TestLintEpicGraph_DependencyOpensLaterOutsideTheMembersWarns is fixture
// (iii): only c, a deferrable epic of A, reaches b, and no member reaches c,
// so A's gate can go green and c can finish once B opens: a warning.
func TestLintEpicGraph_DependencyOpensLaterOutsideTheMembersWarns(t *testing.T) {
	g := opensLaterGraph(dependsOn("a", "A"), dependsOn("c", "A", "b"), dependsOn("b", "B"))
	g.Gates = map[string]model.EpicGate{"A": {Deferrable: []string{"c"}}}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityWarning, found[0].Severity)
	assert.Equal(t, []string{"b", "c"}, found[0].Epics)
	assert.Contains(t, found[0].Message, "c -> b")
}

// TestLintEpicGraph_DependencyOpensLaterSkipsSiblings is fixture (iv): A and
// B both open after nothing, so edges between them either way are clean.
func TestLintEpicGraph_DependencyOpensLaterSkipsSiblings(t *testing.T) {
	g := opensLaterGraph(dependsOn("a", "A", "b"), dependsOn("b", "B"), dependsOn("a2", "A"), dependsOn("b2", "B", "a2"))
	g.OpensAfter = map[string][]string{"A": {}, "B": {}}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater))
}

// TestLintEpicGraph_DependencyOpensLaterSkipsAnEdgeIntoAnAlwaysOpenMilestone
// is fixture (v): on tracks [[A, B], [L]] with L opening after nothing, b in
// B depending on l in L is clean, as is a in A reaching l through b.
func TestLintEpicGraph_DependencyOpensLaterSkipsAnEdgeIntoAnAlwaysOpenMilestone(t *testing.T) {
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"L"}},
		OpensAfter: map[string][]string{"L": {}},
		Epics:      []model.Epic{dependsOn("a", "A"), dependsOn("b", "B", "l", "a"), dependsOn("l", "L")},
	}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater))
}

// TestLintEpicGraph_DependencyOpensLaterIsOneFindingPerGateAndEpic: a1 and
// a2, both members of A's gate, reach b over three paths; that is one error,
// naming both members and b.
func TestLintEpicGraph_DependencyOpensLaterIsOneFindingPerGateAndEpic(t *testing.T) {
	g := opensLaterGraph(dependsOn("a1", "A", "b", "c"), dependsOn("a2", "A", "c"), dependsOn("c", "A", "b"), dependsOn("b", "B"))
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a1", "a2"}}}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, []string{"a1", "a2", "b"}, found[0].Epics)
	assert.Contains(t, found[0].Message, "a1 -> b")
}

// TestLintEpicGraph_DependencyOpensLaterLeavesTrackOrderToLaterMilestoneDependency:
// on track [A, B], a's direct edge to b is later-milestone-dependency's
// error alone; the same reach through x of another track breaks no track
// order, so it is dependency-opens-later's.
func TestLintEpicGraph_DependencyOpensLaterLeavesTrackOrderToLaterMilestoneDependency(t *testing.T) {
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"X"}},
		Epics:      []model.Epic{dependsOn("a", "A", "b"), dependsOn("b", "B")},
	}
	findings := LintEpicGraph(g, nil)
	assert.Len(t, findingsOf(t, findings, EpicRuleLaterMilestoneDependency), 1)
	assert.Empty(t, findingsOf(t, findings, EpicRuleDependencyOpensLater))

	g.Epics = []model.Epic{dependsOn("a", "A", "x"), dependsOn("x", "X", "b"), dependsOn("b", "B")}
	findings = LintEpicGraph(g, nil)
	assert.Empty(t, findingsOf(t, findings, EpicRuleLaterMilestoneDependency))
	found := findingsOf(t, findings, EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityError, found[0].Severity)
	assert.Contains(t, found[0].Message, "a -> x -> b")
}

// TestLintEpicGraph_DependencyOpensLaterFindsALongerWaitCycle: on tracks
// [[A, B], [C, D]], member a of A depends on d in D, which opens after C, and
// member c of C on b in B, which opens after A. No milestone pair waits on
// itself, yet the four wait in one cycle: one error, found from either of its
// two reach edges, in canonical rotation, naming each edge's path.
func TestLintEpicGraph_DependencyOpensLaterFindsALongerWaitCycle(t *testing.T) {
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"C", "D"}},
		Epics:      []model.Epic{dependsOn("a", "A", "d"), dependsOn("b", "B"), dependsOn("c", "C", "b"), dependsOn("d", "D")},
	}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityError, found[0].Severity)
	assert.Equal(t, []string{"a", "b", "c", "d"}, found[0].Epics)
	assert.Equal(t, []string{"A", "B", "C", "D"}, found[0].Names)
	assert.Contains(t, found[0].Message, `"A" -> "D" -> "C" -> "B" -> "A"`)
	assert.Contains(t, found[0].Message, "a -> d")
	assert.Contains(t, found[0].Message, "c -> b")
}

// TestLintEpicGraph_DependencyOpensLaterNamesEachEdgeOfTheCycle: the cycle of
// the longer wait fixture, found first from C's edge since c leads the graph,
// still reads in canonical rotation from A. Each step names the milestone the
// opening waits on next, and A's edge names both members a1 and a2 and the
// first one's whole path, a1 -> x -> y -> d.
func TestLintEpicGraph_DependencyOpensLaterNamesEachEdgeOfTheCycle(t *testing.T) {
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"C", "D"}},
		Gates:      map[string]model.EpicGate{"A": {Members: []string{"a1", "a2"}}},
		Epics: []model.Epic{
			dependsOn("c", "C", "b"), dependsOn("b", "B"), dependsOn("a1", "A", "x"), dependsOn("x", "A", "y"),
			dependsOn("y", "A", "d"), dependsOn("a2", "A", "d"), dependsOn("d", "D"),
		},
	}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Contains(t, found[0].Message, `"A" -> "D" -> "C" -> "B" -> "A"`)
	assert.Contains(t, found[0].Message, `its members a1, a2 reaching d, a1 through a1 -> x -> y -> d; "D" opens after "C"`)
	assert.Contains(t, found[0].Message, `c reaching b through c -> b; "B" opens after "A"`)
}

// TestLintEpicGraph_DependencyOpensLaterStopsAtADoneRecord: reach runs
// through epics without a done record only, so a done epic on the path, a
// done target and a done member each leave the plan clean.
func TestLintEpicGraph_DependencyOpensLaterStopsAtADoneRecord(t *testing.T) {
	done := &model.EpicDone{Evidence: "shipped"}
	through := opensLaterGraph(dependsOn("a", "A", "c"), dependsOn("c", "A", "b"), dependsOn("b", "B"))
	through.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}}}
	through.Epics[1].Done = done
	assert.Empty(t, findingsOf(t, LintEpicGraph(through, nil), EpicRuleDependencyOpensLater), "a done epic on the path cuts it")

	target := opensLaterGraph(dependsOn("a", "A", "b"), dependsOn("b", "B"))
	target.Epics[1].Done = done
	assert.Empty(t, findingsOf(t, LintEpicGraph(target, nil), EpicRuleDependencyOpensLater), "a done target is no wait")

	member := opensLaterGraph(dependsOn("a", "A", "b"), dependsOn("b", "B"))
	member.Epics[0].Done = done
	assert.Empty(t, findingsOf(t, LintEpicGraph(member, nil), EpicRuleDependencyOpensLater), "a done member holds no gate")
}

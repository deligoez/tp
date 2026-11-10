package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// anyOfLintGraph is tracks [[A, B]]: x, y and z root epics of A, and a in B
// depending on the group {x, y}. It lints clean.
func anyOfLintGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}},
		Epics: []model.Epic{
			{ID: "a", Title: "Alpha", Milestone: "B", DependsOn: []model.EpicDep{{AnyOf: []string{"x", "y"}, Why: "either opens the API"}}},
			{ID: "x", Title: "X", Milestone: "A", Root: true},
			{ID: "y", Title: "Y", Milestone: "A", Root: true},
			{ID: "z", Title: "Z", Milestone: "A", Root: true},
		},
	}
}

// TestLintEpicGraph_AnyOfShapeFlagsEachMalformedEntry (spec/1.7.0.md §7):
// fewer than two distinct members, both or neither of id and any_of, a group
// beside a plain edge to one of its members, and two groups of one epic with
// one member set are each one any-of-shape error; a sound group is none.
func TestLintEpicGraph_AnyOfShapeFlagsEachMalformedEntry(t *testing.T) {
	assert.Empty(t, LintEpicGraph(anyOfLintGraph(), nil))
	for name, deps := range map[string][]model.EpicDep{
		"one member":           {{AnyOf: []string{"x"}, Why: "w"}},
		"a repeated member":    {{AnyOf: []string{"x", "x"}, Why: "w"}},
		"no member":            {{AnyOf: []string{}, Why: "w"}},
		"both id and any_of":   {{ID: "z", AnyOf: []string{"x", "y"}, Why: "w"}},
		"neither":              {{Why: "w"}},
		"beside a plain edge":  {{AnyOf: []string{"x", "y"}, Why: "w"}, {ID: "y", Why: "y alone"}},
		"one member set twice": {{AnyOf: []string{"x", "y"}, Why: "w"}, {AnyOf: []string{"y", "x"}, Why: "again"}},
	} {
		g := anyOfLintGraph()
		g.Epics[0].DependsOn = deps
		shape := findingsOf(t, LintEpicGraph(g, nil), EpicRuleAnyOfShape)
		require.Len(t, shape, 1, name)
		assert.Equal(t, EpicSeverityError, shape[0].Severity, name)
		assert.Contains(t, shape[0].Epics, "a", name)
	}
}

// TestLintEpicGraph_DanglingReferenceNamesEachMissingMember: a group member
// that names no epic is a dangling-reference of its own, and the members the
// graph holds are none.
func TestLintEpicGraph_DanglingReferenceNamesEachMissingMember(t *testing.T) {
	g := anyOfLintGraph()
	g.Epics[0].DependsOn[0].AnyOf = []string{"x", "ghost", "phantom"}
	dangling := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDanglingReference)
	require.Len(t, dangling, 2)
	assert.Equal(t, []string{"a", "ghost"}, dangling[0].Epics)
	assert.Equal(t, []string{"a", "phantom"}, dangling[1].Epics)
}

// TestLintEpicGraph_AnEdgeToAnyMemberClosesACycle: a loop through one
// member of a group is a cycle though the other member could be done first,
// and a group naming its own epic is a self-loop.
func TestLintEpicGraph_AnEdgeToAnyMemberClosesACycle(t *testing.T) {
	g := anyOfLintGraph()
	g.Epics[2].Root, g.Epics[2].Milestone = false, "B"
	g.Epics[2].DependsOn = []model.EpicDep{{ID: "a", Why: "y extends a"}}
	cycles := findingsOf(t, LintEpicGraph(g, nil), EpicRuleCycle)
	require.Len(t, cycles, 1)
	assert.Equal(t, []string{"a", "y"}, cycles[0].Epics)

	g = anyOfLintGraph()
	g.Epics[0].DependsOn[0].AnyOf = []string{"a", "x"}
	cycles = findingsOf(t, LintEpicGraph(g, nil), EpicRuleCycle)
	require.Len(t, cycles, 1)
	assert.Equal(t, []string{"a"}, cycles[0].Epics)
}

// TestLintEpicGraph_MissingWhyAsksOneWhyPerGroup: a group without a why is
// one missing-why naming the epic and every member, worded as the group.
func TestLintEpicGraph_MissingWhyAsksOneWhyPerGroup(t *testing.T) {
	g := anyOfLintGraph()
	g.Epics[0].DependsOn[0].Why = " "
	missing := findingsOf(t, LintEpicGraph(g, nil), EpicRuleMissingWhy)
	require.Len(t, missing, 1)
	assert.Equal(t, []string{"a", "x", "y"}, missing[0].Epics)
	assert.Contains(t, missing[0].Message, "one of {x, y}")
}

// TestLintEpicGraph_ThinWhyDoesNotApplyToAGroup: a group's why that
// restates a member's title is no thin-why, while the same why on a plain
// edge to that member is.
func TestLintEpicGraph_ThinWhyDoesNotApplyToAGroup(t *testing.T) {
	g := anyOfLintGraph()
	g.Epics[0].DependsOn[0] = model.EpicDep{ID: "x", AnyOf: []string{"x", "y"}, Why: "X"}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleThinWhy), "an entry carrying any_of is judged as a group")

	g.Epics[0].DependsOn[0] = model.EpicDep{ID: "x", Why: "X"}
	assert.Len(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleThinWhy), 1)
}

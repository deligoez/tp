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

// TestLintEpicGraph_AGroupIsADependencyForNoDependencies: a in B depends on
// nothing but a group, so no-dependencies does not fire on it.
func TestLintEpicGraph_AGroupIsADependencyForNoDependencies(t *testing.T) {
	g := anyOfLintGraph()
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleNoDependencies))
	g.Epics[0].DependsOn = nil
	assert.Len(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleNoDependencies), 1)
}

// TestLintEpicGraph_AGroupIsADependencyForRootWithDependencies: an epic
// marked root whose only entry is a group has a dependency to warn about.
func TestLintEpicGraph_AGroupIsADependencyForRootWithDependencies(t *testing.T) {
	g := anyOfLintGraph()
	g.Epics[0].Root = true
	root := findingsOf(t, LintEpicGraph(g, nil), EpicRuleRootWithDependencies)
	require.Len(t, root, 1)
	assert.Equal(t, []string{"a"}, root[0].Epics)
}

// tagGroupGraph is track [A]: p provides tag ui, q depends on p, r on
// nothing, and the ui carrier s depends on the group {p, q}.
func tagGroupGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}},
		TagChecks:  map[string]model.EpicTagCheck{"ui": {ProvidedBy: "p"}},
		Epics: []model.Epic{
			{ID: "p", Title: "Provider", Milestone: "A", Root: true},
			{ID: "q", Title: "Q", Milestone: "A", DependsOn: []model.EpicDep{{ID: "p", Why: "q calls the checker"}}},
			{ID: "r", Title: "R", Milestone: "A", Root: true},
			{ID: "s", Title: "Screen", Milestone: "A", Tags: []string{"ui"}, DependsOn: []model.EpicDep{{AnyOf: []string{"p", "q"}, Why: "either serves the screen"}}},
		},
	}
}

// TestLintEpicGraph_ATagProviderCountsThroughAGroupOnlyWhenEveryMemberReachesIt:
// {p, q} provides the tag, since p is the provider and q depends on it; a
// member that reaches the provider only through another group counts by the
// same rule; {p, r} does not, since r may be done instead.
func TestLintEpicGraph_ATagProviderCountsThroughAGroupOnlyWhenEveryMemberReachesIt(t *testing.T) {
	assert.Empty(t, findingsOf(t, LintEpicGraph(tagGroupGraph(), nil), EpicRuleUnprovidedTagCheck))

	g := tagGroupGraph()
	g.Epics[2].Root = false
	g.Epics[2].DependsOn = []model.EpicDep{{AnyOf: []string{"p", "q"}, Why: "r reads either"}}
	g.Epics[3].DependsOn[0].AnyOf = []string{"q", "r"}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleUnprovidedTagCheck), "r reaches p through its own group")

	g = tagGroupGraph()
	g.Epics[3].DependsOn[0].AnyOf = []string{"p", "r"}
	unprovided := findingsOf(t, LintEpicGraph(g, nil), EpicRuleUnprovidedTagCheck)
	require.Len(t, unprovided, 1)
	assert.Equal(t, []string{"s"}, unprovided[0].Epics)
}

// TestLintEpicGraph_ADirectTagCheckIgnoresAGroupHoldingTheProvider
// (spec/1.7.0.md §3): only a plain direct edge provides a direct check, so
// an entry carrying any_of does not, even when its id is the provider.
func TestLintEpicGraph_ADirectTagCheckIgnoresAGroupHoldingTheProvider(t *testing.T) {
	g := tagGroupGraph()
	g.TagChecks["ui"] = model.EpicTagCheck{ProvidedBy: "p", Direct: true}
	g.Epics[3].DependsOn[0].ID = "p"
	assert.Len(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleUnprovidedTagCheck), 1)
}

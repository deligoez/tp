package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// deferrableLintGraph is milestone m0 whose gate lists members and deferrable,
// over epics.
func deferrableLintGraph(members, deferrable []string, epics ...model.Epic) *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"m0"}},
		Gates:      map[string]model.EpicGate{"m0": {Members: members, Deferrable: deferrable}},
		Epics:      epics,
	}
}

// TestLintEpicGraph_DeferrableRequiredNamesDeferrableGateAndPath: a1, a
// member of m0's gate, reaches the deferrable d1 through x1, so deferring d1
// changes nothing. One warning naming d1, the gate and the path.
func TestLintEpicGraph_DeferrableRequiredNamesDeferrableGateAndPath(t *testing.T) {
	g := deferrableLintGraph([]string{"a1"}, []string{"d1", "d2"},
		dependsOn("a1", "m0", "x1"), dependsOn("x1", "m0", "d1"), dependsOn("d1", "m0"), dependsOn("d2", "m0"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDeferrableRequired)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityWarning, found[0].Severity)
	assert.Equal(t, []string{"a1", "d1", "x1"}, found[0].Epics, "the deferrable, the example member and every requirer")
	assert.Equal(t, []string{"m0"}, found[0].Names)
	assert.Contains(t, found[0].Message, "deferrable d1")
	assert.Contains(t, found[0].Message, `"m0"`)
	assert.Contains(t, found[0].Message, "required by x1")
	assert.Contains(t, found[0].Message, "a1 -> x1 -> d1")
}

// TestLintEpicGraph_DeferrableRequiredImplicitGate: a gate naming no members
// takes the milestone's other epics, and b1 among them reaches d1.
func TestLintEpicGraph_DeferrableRequiredImplicitGate(t *testing.T) {
	g := deferrableLintGraph(nil, []string{"d1"}, dependsOn("b1", "m0", "d1"), dependsOn("d1", "m0"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDeferrableRequired)
	require.Len(t, found, 1, "%v", found)
	assert.Contains(t, found[0].Message, "b1 -> d1")
}

// TestLintEpicGraph_DeferrableRequiredListsEveryRequirer: a1 needs d1 through
// x1 and y1, and the member a2 needs it directly, so fixing one path leaves
// the warning standing. Every direct requirer in the required set is listed,
// in graph order, with one member path as the example.
func TestLintEpicGraph_DeferrableRequiredListsEveryRequirer(t *testing.T) {
	g := deferrableLintGraph([]string{"a1", "a2"}, []string{"d1"},
		dependsOn("a1", "m0", "x1", "y1"), dependsOn("a2", "m0", "d1"), dependsOn("x1", "m0", "d1"),
		dependsOn("y1", "m0", "d1"), dependsOn("d1", "m0"), dependsOn("z1", "m0", "d1"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDeferrableRequired)
	require.Len(t, found, 1, "%v", found)
	assert.Contains(t, found[0].Message, "required by a2, x1, y1:", "z1 needs d1 but no member reaches z1")
	assert.Contains(t, found[0].Message, "a1 -> x1 -> d1")
	assert.Equal(t, []string{"a1", "a2", "d1", "x1", "y1"}, found[0].Epics)
}

// TestLintEpicGraph_DeferrableRequiredCapsTheRequirers: four requirers are
// counted past the third.
func TestLintEpicGraph_DeferrableRequiredCapsTheRequirers(t *testing.T) {
	g := deferrableLintGraph([]string{"a1"}, []string{"d1"},
		dependsOn("a1", "m0", "w1", "x1", "y1", "z1"), dependsOn("w1", "m0", "d1"), dependsOn("x1", "m0", "d1"),
		dependsOn("y1", "m0", "d1"), dependsOn("z1", "m0", "d1"), dependsOn("d1", "m0"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDeferrableRequired)
	require.Len(t, found, 1, "%v", found)
	assert.Contains(t, found[0].Message, "required by 4 epics (w1, x1, y1, …)")
}

// TestLintEpicGraph_DeferrableRequiredSkips: no warning on an epic listed as
// member and deferrable (gate-member-deferrable's), on a deferrable reached
// only through an any-of alternative, when the member reaching it is done,
// when the only path runs through a done epic (a1 -> x1, done, -> d1), or
// when the deferrable itself is done.
func TestLintEpicGraph_DeferrableRequiredSkips(t *testing.T) {
	doneMember := dependsOn("a1", "m0", "d1")
	doneMember.Done = epicDoneRecord()
	doneX := dependsOn("x1", "m0", "d1")
	doneX.Done = epicDoneRecord()
	doneD := dependsOn("d1", "m0")
	doneD.Done = epicDoneRecord()
	anyOf := dependsOn("a1", "m0")
	anyOf.Root = false
	anyOf.DependsOn = []model.EpicDep{{AnyOf: []string{"d1", "o1"}, Why: "a1 needs one of them"}}
	cases := map[string]*model.EpicGraph{
		"member and deferrable": deferrableLintGraph([]string{"a1", "d1"}, []string{"d1"},
			dependsOn("a1", "m0", "d1"), dependsOn("d1", "m0")),
		"any-of only": deferrableLintGraph([]string{"a1"}, []string{"d1"},
			anyOf, dependsOn("d1", "m0"), dependsOn("o1", "m0")),
		"requiring member done": deferrableLintGraph([]string{"a1"}, []string{"d1"},
			doneMember, dependsOn("d1", "m0")),
		"done intermediate": deferrableLintGraph([]string{"a1"}, []string{"d1"},
			dependsOn("a1", "m0", "x1"), doneX, dependsOn("d1", "m0")),
		"done deferrable": deferrableLintGraph([]string{"a1"}, []string{"d1"},
			dependsOn("a1", "m0", "d1"), doneD),
	}
	for name, g := range cases {
		assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleDeferrableRequired), name)
	}
}

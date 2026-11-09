package engine

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// cleanEpicGraph is the chain a → b → c over tracks [[A, B], [L1]], with a
// gate and a tag check that both name real epics.
func cleanEpicGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"L1"}},
		Gates:      map[string]model.EpicGate{"A": {Members: []string{"a"}, Deferrable: []string{"b"}}},
		TagChecks:  map[string]model.EpicTagCheck{"ui": {ProvidedBy: "c"}},
		Epics: []model.Epic{
			{ID: "a", Title: "A", Milestone: "A", Tags: []string{"ui"}, DependsOn: []model.EpicDep{{ID: "b", Why: "b first"}}},
			{ID: "b", Title: "B", Milestone: "A", DependsOn: []model.EpicDep{{ID: "c", Why: "c first"}}},
			{ID: "c", Title: "C", Milestone: "A"},
		},
	}
}

// findingsOf returns the findings with the given rule, after checking that
// every finding carries a rule, a severity, a message and sorted epics.
func findingsOf(t *testing.T, findings []EpicLintFinding, rule string) []EpicLintFinding {
	t.Helper()
	var out []EpicLintFinding
	for _, f := range findings {
		assert.NotEmpty(t, f.Rule)
		assert.Contains(t, []string{EpicSeverityError, EpicSeverityWarning}, f.Severity)
		assert.NotEmpty(t, f.Message)
		require.NotNil(t, f.Epics, "epics is an array, never null")
		assert.True(t, sort.StringsAreSorted(f.Epics), "epics are sorted: %v", f.Epics)
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestLintEpicGraph_CleanGraphHasNoStructureFindings(t *testing.T) {
	assert.Empty(t, LintEpicGraph(cleanEpicGraph(), nil))
}

func TestLintEpicGraph_OverlappingCyclesAreOneFinding(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics[1].DependsOn = append(g.Epics[1].DependsOn, model.EpicDep{ID: "a", Why: "back to a"})
	g.Epics[2].DependsOn = []model.EpicDep{{ID: "b", Why: "back to b"}}
	cycles := findingsOf(t, LintEpicGraph(g, nil), EpicRuleCycle)
	require.Len(t, cycles, 1)
	assert.Equal(t, []string{"a", "b", "c"}, cycles[0].Epics)
}

func TestLintEpicGraph_SelfLoopIsACycle(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics[2].DependsOn = []model.EpicDep{{ID: "c", Why: "itself"}}
	cycles := findingsOf(t, LintEpicGraph(g, nil), EpicRuleCycle)
	require.Len(t, cycles, 1)
	assert.Equal(t, []string{"c"}, cycles[0].Epics)
}

func TestLintEpicGraph_DanglingReferencesAreOneFindingEach(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics[2].DependsOn = []model.EpicDep{{ID: "z", Why: "absent"}}
	g.Gates["A"] = model.EpicGate{Members: []string{"y"}, Deferrable: []string{"x"}}
	g.Gates["Q"] = model.EpicGate{Members: []string{"a"}}
	g.TagChecks["ui"] = model.EpicTagCheck{ProvidedBy: "w"}
	dangling := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDanglingReference)
	require.Len(t, dangling, 5)
	var epics [][]string
	for _, f := range dangling {
		epics = append(epics, f.Epics)
	}
	assert.ElementsMatch(t, [][]string{{"c", "z"}, {"y"}, {"x"}, {"w"}, {}}, epics)
}

func TestLintEpicGraph_DuplicateNamesAreOneFindingEach(t *testing.T) {
	g := cleanEpicGraph()
	g.Milestones = [][]string{{"A", "B"}, {"L1", "B"}}
	g.Epics = append(g.Epics, model.Epic{ID: "c", Title: "C again", Milestone: "A"}, model.Epic{ID: "c", Title: "C thrice", Milestone: "A"})
	dups := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDuplicateName)
	require.Len(t, dups, 2)
	assert.Equal(t, []string{"c"}, dups[0].Epics)
	assert.Equal(t, []string{}, dups[1].Epics)
	assert.Contains(t, dups[1].Message, `"B"`)
}

func TestLintEpicGraph_UnknownKeysAreOneFindingEach(t *testing.T) {
	unknown := []EpicGraphUnknownKey{
		{Path: "owner", Key: "owner"},
		{Path: "epics[1].trigger", Key: "trigger", EpicID: "b"},
	}
	keys := findingsOf(t, LintEpicGraph(cleanEpicGraph(), unknown), EpicRuleUnknownKey)
	require.Len(t, keys, 2)
	assert.Equal(t, []string{}, keys[0].Epics)
	assert.Contains(t, keys[0].Message, "owner")
	assert.Equal(t, []string{"b"}, keys[1].Epics)
	assert.Contains(t, keys[1].Message, "epics[1].trigger")
}

func TestLintEpicGraph_LaterMilestoneDependencyFiresWithinATrackOnly(t *testing.T) {
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"L1"}},
		Epics: []model.Epic{
			{ID: "a", Title: "A", Milestone: "A", DependsOn: []model.EpicDep{{ID: "b", Why: "b first"}, {ID: "l", Why: "l first"}}},
			{ID: "b", Title: "B", Milestone: "B"},
			{ID: "l", Title: "L", Milestone: "L1"},
		},
	}
	later := findingsOf(t, LintEpicGraph(g, nil), EpicRuleLaterMilestoneDependency)
	require.Len(t, later, 1, "the edge to l in another track is free")
	assert.Equal(t, []string{"a", "b"}, later[0].Epics)
}

func TestLintEpicGraph_MissingWhyFiresOnAbsentAndBlankWhy(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics[0].DependsOn = []model.EpicDep{{ID: "b"}}
	g.Epics[1].DependsOn = []model.EpicDep{{ID: "c", Why: " \t\n "}}
	missing := findingsOf(t, LintEpicGraph(g, nil), EpicRuleMissingWhy)
	require.Len(t, missing, 2)
	assert.Equal(t, []string{"a", "b"}, missing[0].Epics)
	assert.Equal(t, []string{"b", "c"}, missing[1].Epics)
}

func TestLintEpicGraph_UnknownMilestoneFiresOnAbsentAndUntrackedMilestone(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics[1].Milestone = ""
	g.Epics[2].Milestone = "Z"
	unknown := findingsOf(t, LintEpicGraph(g, nil), EpicRuleUnknownMilestone)
	require.Len(t, unknown, 2)
	assert.Equal(t, []string{"b"}, unknown[0].Epics)
	assert.Equal(t, []string{"c"}, unknown[1].Epics)
	assert.Contains(t, unknown[1].Message, `"Z"`)
}

func TestLintEpicGraph_GateMemberOutsideMilestoneFiresOnTheMember(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics = append(g.Epics, model.Epic{ID: "d", Title: "D", Milestone: "B"})
	g.Gates["A"] = model.EpicGate{Members: []string{"a", "d"}}
	outside := findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateMemberOutsideMilestone)
	require.Len(t, outside, 1)
	assert.Equal(t, []string{"d"}, outside[0].Epics)
	assert.Contains(t, outside[0].Message, `"A"`)
}

// providerGraph is section 9 row 4's fixture: p tagged ui and named its
// provider, x → p, and screen tagged ui with screen → x.
func providerGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}},
		TagChecks:  map[string]model.EpicTagCheck{"ui": {ProvidedBy: "p"}},
		Epics: []model.Epic{
			{ID: "p", Title: "Provider", Milestone: "A", Tags: []string{"ui"}},
			{ID: "x", Title: "Middle", Milestone: "A", DependsOn: []model.EpicDep{{ID: "p", Why: "needs the checker"}}},
			{ID: "screen", Title: "Screen", Milestone: "A", Tags: []string{"ui"}, DependsOn: []model.EpicDep{{ID: "x", Why: "renders its data"}}},
		},
	}
}

func TestLintEpicGraph_UnprovidedTagCheckFollowsTransitiveDependencies(t *testing.T) {
	assert.Empty(t, LintEpicGraph(providerGraph(), nil), "a transitive path and the provider itself do not fire")

	g := providerGraph()
	g.Epics[2].DependsOn = nil
	unprovided := findingsOf(t, LintEpicGraph(g, nil), EpicRuleUnprovidedTagCheck)
	require.Len(t, unprovided, 1)
	assert.Equal(t, []string{"screen"}, unprovided[0].Epics)
	assert.Equal(t, EpicSeverityError, unprovided[0].Severity)
}

// severityCount counts the findings at the given severity.
func severityCount(findings []EpicLintFinding, severity string) int {
	n := 0
	for _, f := range findings {
		if f.Severity == severity {
			n++
		}
	}
	return n
}

// warningGraph is section 9 row 23's fixture as D10 fixes it: tracks [[A, B]],
// a and c in A, b in B with no dependencies, a → c whose why is " STEP " while
// c's title is "c step".
func warningGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}},
		Epics: []model.Epic{
			{ID: "a", Title: "First", Milestone: "A", DependsOn: []model.EpicDep{{ID: "c", Why: " STEP "}}},
			{ID: "b", Title: "Later", Milestone: "B"},
			{ID: "c", Title: "c step", Milestone: "A"},
		},
	}
}

func TestLintEpicGraph_OneInstanceOfEachWarning(t *testing.T) {
	g := warningGraph()
	findings := LintEpicGraph(g, nil)
	assert.Equal(t, 2, severityCount(findings, EpicSeverityWarning))
	assert.Equal(t, 0, severityCount(findings, EpicSeverityError))

	noDeps := findingsOf(t, findings, EpicRuleNoDependencies)
	require.Len(t, noDeps, 1)
	assert.Equal(t, []string{"b"}, noDeps[0].Epics)
	thin := findingsOf(t, findings, EpicRuleThinWhy)
	require.Len(t, thin, 1)
	assert.Equal(t, []string{"a", "c"}, thin[0].Epics)

	// The fixture separates the folded comparison from a case-sensitive one,
	// which would find "STEP" in neither "c" nor "c step" and yield warnings 1.
	why := strings.TrimSpace(g.Epics[0].DependsOn[0].Why)
	assert.False(t, strings.Contains(g.Epics[2].ID, why) || strings.Contains(g.Epics[2].Title, why))
}

func TestLintEpicGraph_EmptyWhyDoesNotFireThinWhy(t *testing.T) {
	g := warningGraph()
	g.Epics[0].DependsOn[0].Why = "  "
	findings := LintEpicGraph(g, nil)
	assert.Empty(t, findingsOf(t, findings, EpicRuleThinWhy), "an empty why is missing-why's finding")
	assert.Len(t, findingsOf(t, findings, EpicRuleMissingWhy), 1)
}

// TestLintEpicGraph_GateKeyedByTheNextMilestoneHintsTheRightKey: a gate whose
// members all sit in one other milestone was keyed by the milestone it opens;
// the message names the key the members' milestone would take.
func TestLintEpicGraph_GateKeyedByTheNextMilestoneHintsTheRightKey(t *testing.T) {
	g := cleanEpicGraph()
	g.Epics = append(g.Epics, model.Epic{ID: "d", Title: "D", Milestone: "B"})
	g.Gates["A"] = model.EpicGate{Members: []string{"d"}}
	outside := findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateMemberOutsideMilestone)
	require.Len(t, outside, 1)
	assert.Contains(t, outside[0].Message, `gates."B"`)

	g.Gates["A"] = model.EpicGate{Members: []string{"a", "d"}}
	mixed := findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateMemberOutsideMilestone)
	require.Len(t, mixed, 1)
	assert.NotContains(t, mixed[0].Message, "did you mean", "members in two milestones name no single key")
}

// TestLintEpicGraph_ThinWhyMatchesWholeWordsOnly: a why that is only a
// prefix of a word in the target's title is not a restatement of it.
func TestLintEpicGraph_ThinWhyMatchesWholeWordsOnly(t *testing.T) {
	for _, tc := range []struct {
		why  string
		fire bool
	}{
		{"ident", false},
		{"Service", true},
		{"identity service", true},
		{"auth", true},
		{"needs the token store", false},
	} {
		t.Run(tc.why, func(t *testing.T) {
			g := warningGraph()
			g.Epics[2].ID = "auth-core"
			g.Epics[2].Title = "Identity service"
			g.Epics[0].DependsOn[0] = model.EpicDep{ID: "auth-core", Why: tc.why}
			thin := findingsOf(t, LintEpicGraph(g, nil), EpicRuleThinWhy)
			assert.Equal(t, tc.fire, len(thin) == 1, "why %q", tc.why)
		})
	}
}

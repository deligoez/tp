package engine

import (
	"sort"
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
		assert.Equal(t, EpicSeverityError, f.Severity)
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


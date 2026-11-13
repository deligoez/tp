package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// triggerGateGraph is the shape that hides an empty gate: TG holds only the
// condition trigger and has no gates entry, so its gate has no effective
// members, and T1 opens after TG.
func triggerGateGraph() *model.EpicGraph {
	trigger := dependsOn("trigger", "TG")
	trigger.Condition, trigger.Acceptance = true, "the vendor signs"
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}, {"TG", "T1"}},
		OpensAfter: map[string][]string{"TG": {}, "T1": {"TG"}},
		Epics:      []model.Epic{dependsOn("a", "A"), trigger, dependsOn("t1", "T1")},
	}
}

// TestLintEpicGraph_GateEmptyWarnsWhenAMilestoneOpensAfterIt: TG's gate has
// no effective members and no threshold, so it is green at once and T1 opens
// without waiting on the trigger. One warning on TG, naming T1.
func TestLintEpicGraph_GateEmptyWarnsWhenAMilestoneOpensAfterIt(t *testing.T) {
	found := findingsOf(t, LintEpicGraph(triggerGateGraph(), nil), EpicRuleGateEmpty)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityWarning, found[0].Severity)
	assert.Equal(t, []string{"trigger"}, found[0].Epics)
	assert.Equal(t, []string{"TG"}, found[0].Names)
	assert.Contains(t, found[0].Message, `"T1" opens after it`)
}

// TestLintEpicGraph_GateEmptySkipsAThresholdOrMembers: a threshold holds
// TG's gate until a met record, and an explicit member holds it until that
// epic is done, so neither gate is empty.
func TestLintEpicGraph_GateEmptySkipsAThresholdOrMembers(t *testing.T) {
	for name, gate := range map[string]model.EpicGate{
		"threshold": {Threshold: "the vendor contract is signed"},
		"members":   {Members: []string{"trigger"}},
	} {
		g := triggerGateGraph()
		g.Gates = map[string]model.EpicGate{"TG": gate}
		assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateEmpty), name)
	}
}

// TestLintEpicGraph_GateEmptySkipsAMilestoneNothingOpensAfter: T1 opens
// after nothing, so TG's empty gate holds nothing back.
func TestLintEpicGraph_GateEmptySkipsAMilestoneNothingOpensAfter(t *testing.T) {
	g := triggerGateGraph()
	g.OpensAfter["T1"] = []string{}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateEmpty))
}

// TestLintEpicGraph_GateEmptySkipsAMilestoneWithNoEpics: TG holds no epic
// yet, so nothing in it looks like it holds T1.
func TestLintEpicGraph_GateEmptySkipsAMilestoneWithNoEpics(t *testing.T) {
	g := triggerGateGraph()
	g.Epics = []model.Epic{dependsOn("a", "A"), dependsOn("t1", "T1")}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateEmpty))
}

// TestLintEpicGraph_GateEmptyNamesEveryFollower: T1 and T2 both open after
// TG; one finding names both.
func TestLintEpicGraph_GateEmptyNamesEveryFollower(t *testing.T) {
	g := triggerGateGraph()
	g.Milestones[1] = append(g.Milestones[1], "T2")
	g.OpensAfter["T2"] = []string{"TG"}
	g.Epics = append(g.Epics, dependsOn("t2", "T2"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateEmpty)
	require.Len(t, found, 1, "%v", found)
	assert.Contains(t, found[0].Message, `"T1", "T2" open after it`)
}

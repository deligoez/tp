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
	assert.Contains(t, found[0].Message, `"T1"`)
}


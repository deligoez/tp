package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestLintEpicGraph_GateHeldByConditionNamesMemberConditionAndPath: a, a
// member of A's gate, waits on the undone condition c of the earlier P through
// w; it does not wait on c2, which is done, nor on c3, which it reaches only
// through the done d. One info finding, for (a, c).
func TestLintEpicGraph_GateHeldByConditionNamesMemberConditionAndPath(t *testing.T) {
	cond := func(id string) model.Epic {
		e := dependsOn(id, "P")
		e.Condition, e.Acceptance = true, "the operator decides"
		return e
	}
	done := cond("c2")
	done.Done = epicDoneRecord()
	d := dependsOn("d", "A", "c3")
	d.Done = epicDoneRecord()
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"P", "A"}},
		Gates:      map[string]model.EpicGate{"A": {Members: []string{"a"}}},
		Epics:      []model.Epic{dependsOn("a", "A", "w", "c2", "d"), dependsOn("w", "A", "c"), cond("c"), done, d, cond("c3")},
	}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateHeldByCondition)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityInfo, found[0].Severity)
	assert.Equal(t, []string{"a", "c"}, found[0].Epics)
	assert.Equal(t, []string{"A"}, found[0].Names)
	assert.Contains(t, found[0].Message, "a -> w -> c")
}

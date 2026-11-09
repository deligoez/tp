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

// TestLintEpicGraph_GateHeldByConditionSkipsOwnMilestoneCondition: A is led
// by its own trigger t, a condition in A that every member waits on. That is
// the intended shape of a trigger-led milestone, not a finding.
func TestLintEpicGraph_GateHeldByConditionSkipsOwnMilestoneCondition(t *testing.T) {
	trigger := dependsOn("t", "A")
	trigger.Condition, trigger.Acceptance = true, "the operator decides"
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}},
		Gates:      map[string]model.EpicGate{"A": {Members: []string{"a", "b"}}},
		Epics:      []model.Epic{dependsOn("a", "A", "t"), dependsOn("b", "A", "a"), trigger},
	}
	assert.Empty(t, findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateHeldByCondition))
}

// TestLintEpicGraph_GateHeldByConditionGroupsMembersPerCondition: a and b,
// both members of A's gate, reach the condition c of the earlier P. One
// finding for (A, c) names both members and one example path.
func TestLintEpicGraph_GateHeldByConditionGroupsMembersPerCondition(t *testing.T) {
	c := dependsOn("c", "P")
	c.Condition, c.Acceptance = true, "the operator decides"
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"P", "A"}},
		Gates:      map[string]model.EpicGate{"A": {Members: []string{"a", "b"}}},
		Epics:      []model.Epic{dependsOn("a", "A", "c"), dependsOn("b", "A", "a"), c},
	}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleGateHeldByCondition)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, []string{"a", "b", "c"}, found[0].Epics)
	assert.Equal(t, []string{"A"}, found[0].Names)
	assert.Contains(t, found[0].Message, "a, b")
	assert.Contains(t, found[0].Message, "a -> c")
}

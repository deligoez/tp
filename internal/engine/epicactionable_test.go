package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// conditionActionableOf runs ConditionsNotActionable over g with the epics
// of done done and bound as the bound-file index, and reports whether c is
// listed with no reason.
func conditionActionableOf(g *model.EpicGraph, bound map[string][]string, done ...string) bool {
	states := map[string]EpicState{}
	for i := range g.Epics {
		states[g.Epics[i].ID] = EpicState{Status: EpicStatusPlanned}
	}
	for _, id := range done {
		states[id] = EpicState{Status: EpicStatusDone}
	}
	gates := MilestoneGates(g, states)
	reasons, undone := ConditionsNotActionable(g, states, gates, bound)["c"]
	return undone && len(reasons) == 0
}

// closedDependentGraph is track [A, B]: the condition c and the undone work
// epic a in A, so B stays closed, and w in B waiting on c alone.
func closedDependentGraph() *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "A", Root: true},
		{ID: "c", Title: "C", Milestone: "A", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "w", Title: "W", Milestone: "B", DependsOn: []model.EpicDep{{ID: "c", Why: "w ships once the board agrees"}}},
	}}
}

// TestConditionsActionable_ADoneDependentDoesNotCount: w waits on c alone,
// but w is done, so deciding c moves nothing.
func TestConditionsActionable_ADoneDependentDoesNotCount(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}, Epics: []model.Epic{
		{ID: "c", Title: "C", Milestone: "A", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "w", Title: "W", Milestone: "A", DependsOn: []model.EpicDep{{ID: "c", Why: "w ships once the board agrees"}}},
		{ID: "x", Title: "X", Milestone: "A", Root: true},
	}}
	assert.False(t, conditionActionableOf(g, nil, "w"))
}

// TestConditionsActionable_ADependentInAClosedMilestoneWithoutABoundFileDoesNotCount:
// w waits on c alone, but B is closed and w has no bound file, so nothing can
// start once c is decided.
func TestConditionsActionable_ADependentInAClosedMilestoneWithoutABoundFileDoesNotCount(t *testing.T) {
	assert.False(t, conditionActionableOf(closedDependentGraph(), nil))
}

// TestConditionsActionable_ADependentInAClosedMilestoneWithABoundFileCounts:
// the same w with a bound file is worked through its task file, so c holds
// started work.
func TestConditionsActionable_ADependentInAClosedMilestoneWithABoundFileCounts(t *testing.T) {
	bound := map[string][]string{"w": {"/repo/w.tasks.json"}}
	assert.True(t, conditionActionableOf(closedDependentGraph(), bound))
}

// TestConditionsActionable_EveryDependentDoneIsNotActionable: c gates w1 and
// w2, both done, so c gates nothing.
func TestConditionsActionable_EveryDependentDoneIsNotActionable(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}, Epics: []model.Epic{
		{ID: "c", Title: "C", Milestone: "A", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "w1", Title: "W1", Milestone: "A", DependsOn: []model.EpicDep{{ID: "c", Why: "w1 ships once the board agrees"}}},
		{ID: "w2", Title: "W2", Milestone: "A", DependsOn: []model.EpicDep{{ID: "c", Why: "w2 ships once the board agrees"}}},
	}}
	assert.False(t, conditionActionableOf(g, nil, "w1", "w2"))
}

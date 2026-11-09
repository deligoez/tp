package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// conditionActionableOf runs ConditionsActionable over g with the epics of
// done done and bound as the bound-file index, and returns c's answer.
func conditionActionableOf(g *model.EpicGraph, bound map[string][]string, done ...string) bool {
	states := map[string]EpicState{}
	for i := range g.Epics {
		states[g.Epics[i].ID] = EpicState{Status: EpicStatusPlanned}
	}
	for _, id := range done {
		states[id] = EpicState{Status: EpicStatusDone}
	}
	gates := MilestoneGates(g, states)
	return ConditionsActionable(g, states, gates, bound)["c"]
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


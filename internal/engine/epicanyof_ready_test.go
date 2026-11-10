package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// anyOfStates returns every epic of g planned, those of done done.
func anyOfStates(g *model.EpicGraph, done ...string) map[string]EpicState {
	states := map[string]EpicState{}
	for i := range g.Epics {
		states[g.Epics[i].ID] = EpicState{Status: EpicStatusPlanned}
	}
	for _, id := range done {
		states[id] = EpicState{Status: EpicStatusDone}
	}
	return states
}

// actionableOf reports whether the condition id of g is listed by
// ConditionsNotActionable with no reason, the epics of done done.
func actionableOf(g *model.EpicGraph, id string, done ...string) bool {
	states := anyOfStates(g, done...)
	reasons, undone := ConditionsNotActionable(g, states, MilestoneGates(g, states), nil)[id]
	return undone && len(reasons) == 0
}

// TestEpicReady_OneDoneMemberSatisfiesAGroup is spec/1.7.0.md §7: w depends
// on one of {a, b}; it is no candidate while neither is done and ready once
// a is. A readiness that read a group as an edge to "" never readies w.
func TestEpicReady_OneDoneMemberSatisfiesAGroup(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "M", Root: true},
		{ID: "b", Title: "B", Milestone: "M", Root: true},
		{ID: "w", Title: "W", Milestone: "M", DependsOn: []model.EpicDep{{AnyOf: []string{"a", "b"}, Why: "either opens the API"}}},
	}}
	assert.NotContains(t, EpicReady(g, anyOfStates(g), nil, false).Ready, "w")
	assert.Contains(t, EpicReady(g, anyOfStates(g, "a"), nil, false).Ready, "w")
}

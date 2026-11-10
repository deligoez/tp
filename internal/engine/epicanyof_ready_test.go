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

// TestConditionsActionable_AGroupWithAnOpenConditionMemberCountsAsACondition
// is spec/1.7.0.md §7: x, a gate member of the open M1, depends on one of
// {c, b}, the condition c in M1 and the work b in the closed M2. c has its
// milestone open and its dependencies done, so the group counts as a
// condition and x waits on conditions alone: c is actionable. A group read
// as work leaves c with no dependent that waits on conditions alone.
func TestConditionsActionable_AGroupWithAnOpenConditionMemberCountsAsACondition(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M1", "M2"}}, Epics: []model.Epic{
		{ID: "x", Title: "X", Milestone: "M1", DependsOn: []model.EpicDep{{AnyOf: []string{"c", "b"}, Why: "either opens the API"}}},
		{ID: "c", Title: "C", Milestone: "M1", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "b", Title: "B", Milestone: "M2", Root: true},
	}}
	assert.True(t, actionableOf(g, "c"))
}

// TestConditionsActionable_AGroupWhoseConditionMembersAreClosedIsNoCondition
// is spec/1.7.0.md §7: x in the open M1 depends on c2, an open condition in
// M1, and on one of {c, b}, the condition c and the work b both in the
// closed M2. No condition member of the group can be decided now, so x does
// not wait on conditions alone and c2 is not actionable on its account. A
// group counted as a condition for any condition member makes c2 actionable.
func TestConditionsActionable_AGroupWhoseConditionMembersAreClosedIsNoCondition(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M1", "M2"}}, Epics: []model.Epic{
		{ID: "x", Title: "X", Milestone: "M1", DependsOn: []model.EpicDep{
			{ID: "c2", Why: "x needs the budget"},
			{AnyOf: []string{"c", "b"}, Why: "either opens the API"},
		}},
		{ID: "c2", Title: "C2", Milestone: "M1", Root: true, Condition: true, Acceptance: "the budget is approved"},
		{ID: "c", Title: "C", Milestone: "M2", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "b", Title: "B", Milestone: "M2", Root: true},
	}}
	g.Gates = map[string]model.EpicGate{"M1": {Members: []string{"x"}}}
	assert.False(t, actionableOf(g, "c2"))
}

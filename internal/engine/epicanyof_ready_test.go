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

// TestConditionsActionable_ASatisfiedGroupLeavesItsOtherMembers is
// spec/1.7.0.md §7: w in the open A depends on one of {c1, c2} and on the
// condition c3; c1 is done, so the group is satisfied and w leaves c2's
// actionability. c2's other dependent y sits in the closed B, so nothing
// deciding c2 moves: it is not actionable. A satisfied group still counted
// makes w, which waits on c3 alone, a dependent c2 unblocks.
func TestConditionsActionable_ASatisfiedGroupLeavesItsOtherMembers(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		{ID: "w", Title: "W", Milestone: "A", DependsOn: []model.EpicDep{
			{AnyOf: []string{"c1", "c2"}, Why: "either opens the API"},
			{ID: "c3", Why: "w needs the budget"},
		}},
		{ID: "c1", Title: "C1", Milestone: "A", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "c2", Title: "C2", Milestone: "A", Root: true, Condition: true, Acceptance: "the vendor says yes"},
		{ID: "c3", Title: "C3", Milestone: "A", Root: true, Condition: true, Acceptance: "the budget is approved"},
		{ID: "y", Title: "Y", Milestone: "B", DependsOn: []model.EpicDep{{ID: "c2", Why: "y reads the vendor's terms"}}},
	}}
	assert.False(t, actionableOf(g, "c2", "c1"))
}

// TestEpicReady_ALostAlternativeIsMoot is spec/1.7.0.md §7: w depends on one
// of {c1, c2} and c1 is recorded, so c2 is moot: it is in neither the
// operator's conditions, even with all, nor ConditionsNotActionable's
// answer. A moot epic left in the answer lists c2 as actionable, since
// nothing unsatisfied depends on it.
func TestEpicReady_ALostAlternativeIsMoot(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "w", Title: "W", Milestone: "M", DependsOn: []model.EpicDep{{AnyOf: []string{"c1", "c2"}, Why: "either opens the API"}}},
		{ID: "c1", Title: "C1", Milestone: "M", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "c2", Title: "C2", Milestone: "M", Root: true, Condition: true, Acceptance: "the vendor says yes"},
	}}
	r := EpicReady(g, anyOfStates(g, "c1"), nil, true)
	assert.NotContains(t, r.Conditions, "c2")
	assert.NotContains(t, r.NotActionable, "c2")
	assert.True(t, r.Moot["c2"])
	assert.Contains(t, r.Ready, "w")
}

// TestMootEpics_WhatOnlyALostAlternativeNeedsIsMootToo is spec/1.7.0.md §7:
// w depends on one of {a, b} and a is done, so b is moot; b's plain edge
// to the condition c, which no gate holds, makes c moot too, though c comes
// first in the graph. A rule that judged each epic once, in graph order,
// leaves c unmoot; c then stays among the operator's conditions.
func TestMootEpics_WhatOnlyALostAlternativeNeedsIsMootToo(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "c", Title: "C", Milestone: "M", Root: true, Condition: true, Acceptance: "the vendor says yes"},
		{ID: "w", Title: "W", Milestone: "M", DependsOn: []model.EpicDep{{AnyOf: []string{"a", "b"}, Why: "either opens the API"}}},
		{ID: "a", Title: "A", Milestone: "M", Root: true},
		{ID: "b", Title: "B", Milestone: "M", DependsOn: []model.EpicDep{{ID: "c", Why: "b runs on the vendor's API"}}},
	}}
	g.Gates = map[string]model.EpicGate{"M": {Members: []string{"w", "a"}}}
	states := anyOfStates(g, "a")
	assert.Equal(t, map[string]bool{"b": true, "c": true}, MootEpics(g, states))
	assert.NotContains(t, EpicReady(g, states, nil, true).Conditions, "c")
}

// TestMootEpics_APlainEdgeFromAnEpicThatIsNotMootUnmootsIt is spec/1.7.0.md
// §7: c2 is moot while only the satisfied group of w names it, and stops
// being moot once y, which is not moot, gains a plain edge to it: moot is
// derived on every read, and c2 is back among the operator's conditions. A
// rule that ignored plain edges keeps c2 moot.
func TestMootEpics_APlainEdgeFromAnEpicThatIsNotMootUnmootsIt(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "w", Title: "W", Milestone: "M", DependsOn: []model.EpicDep{{AnyOf: []string{"c1", "c2"}, Why: "either opens the API"}}},
		{ID: "c1", Title: "C1", Milestone: "M", Root: true, Condition: true, Acceptance: "the board says yes"},
		{ID: "c2", Title: "C2", Milestone: "M", Root: true, Condition: true, Acceptance: "the vendor says yes"},
		{ID: "y", Title: "Y", Milestone: "M", Root: true},
	}}
	assert.True(t, MootEpics(g, anyOfStates(g, "c1"))["c2"])

	g.Epics[3].Root = false
	g.Epics[3].DependsOn = []model.EpicDep{{ID: "c2", Why: "y reads the vendor's terms"}}
	states := anyOfStates(g, "c1")
	assert.False(t, MootEpics(g, states)["c2"])
	assert.Contains(t, EpicReady(g, states, nil, false).Conditions, "c2")
}

// TestMootEpics_AnEpicAGateHoldsIsNeverMoot is spec/1.7.0.md §7: b lost to a
// in w's group, but b is a member of M's default gate, so it is not moot
// and stays the operator's to defer or remove. A rule that skipped the gate
// check makes b moot.
func TestMootEpics_AnEpicAGateHoldsIsNeverMoot(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "w", Title: "W", Milestone: "M", DependsOn: []model.EpicDep{{AnyOf: []string{"a", "b"}, Why: "either opens the API"}}},
		{ID: "a", Title: "A", Milestone: "M", Root: true},
		{ID: "b", Title: "B", Milestone: "M", Root: true},
	}}
	assert.Empty(t, MootEpics(g, anyOfStates(g, "a")))
}

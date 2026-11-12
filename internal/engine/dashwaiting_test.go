package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// deferrableGraph is spec/1.8.0.md §18 row 7's deferrable half: M's gate
// lists f as deferrable, beside its members r (done), w (ready) and v
// (blocked by w). f waits on the condition c, whose only dependent it is;
// free is a condition nothing depends on, and dc a deferrable condition.
func deferrableGraph() *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}},
		Gates: map[string]model.EpicGate{"M": {Deferrable: []string{"f", "dc"}}},
		Epics: []model.Epic{
			{ID: "r", Title: "R", Milestone: "M", Root: true},
			{ID: "w", Title: "W", Milestone: "M", DependsOn: []model.EpicDep{{ID: "r", Why: "r first"}}},
			{ID: "v", Title: "V", Milestone: "M", DependsOn: []model.EpicDep{{ID: "w", Why: "w first"}}},
			{ID: "c", Title: "C", Milestone: "M", Condition: true, Acceptance: "x", DependsOn: []model.EpicDep{{ID: "r", Why: "r first"}}},
			{ID: "f", Title: "F", Milestone: "M", DependsOn: []model.EpicDep{{ID: "c", Why: "c first"}}},
			{ID: "free", Title: "Free", Milestone: "M", Condition: true, Acceptance: "x", Root: true},
			{ID: "dc", Title: "DC", Milestone: "M", Condition: true, Acceptance: "x", Root: true},
		}}
}

// TestIsDeferrableMember: an epic its own milestone's gate lists deferrable
// is one; a member, and an epic another milestone's gate lists, are not.
func TestIsDeferrableMember(t *testing.T) {
	g := deferrableGraph()
	assert.True(t, IsDeferrableMember(g, &g.Epics[4]), "f")
	assert.False(t, IsDeferrableMember(g, &g.Epics[2]), "v is a member")
	other := &model.Epic{ID: "f", Milestone: "N"}
	assert.False(t, IsDeferrableMember(g, other), "only its own milestone's gate counts")
}

// TestDashboardWaiting_LeavesDeferrableMembersOut is §7 and §18 row 7: the
// dashboard's waiting list holds v, blocked by w, and not f, which a
// condition blocks but which its gate lists deferrable; no condition is
// listed. A list built from the blocked figure alone carries f.
func TestDashboardWaiting_LeavesDeferrableMembersOut(t *testing.T) {
	g := deferrableGraph()
	states := anyOfStates(g, "r")
	assert.Equal(t, []string{"v"}, DashboardWaiting(g, states, "M"))
	assert.Equal(t, []string{"v"}, DashboardWaiting(g, states, ""), "an empty milestone reads the whole graph")
	assert.Equal(t, 2, WorkFigures(g, states, nil, "M").Blocked, "the CLI figure keeps counting f")
}

// TestEpicReady_LeavesConditionsOnlyDeferrableWorkWaitsOn is §7: of the
// conditions c, dc and free, tp epic ready's operator conditions, which the
// dashboard's operator rows show, hold free alone: dc is a deferrable member,
// and c's only undone dependent is the deferrable f.
func TestEpicReady_LeavesConditionsOnlyDeferrableWorkWaitsOn(t *testing.T) {
	g := deferrableGraph()
	states := anyOfStates(g, "r")
	assert.Equal(t, []string{"free"}, EpicReady(g, states, nil, false).Conditions)
	reasons := ConditionsNotActionable(g, states, MilestoneGates(g, states), nil)
	assert.Equal(t, []string{NotActionableDependents}, reasons["c"])
	assert.Equal(t, []string{NotActionableDeferrable}, reasons["dc"])
}

// TestEpicReady_KeepsAConditionWorkWaitsOn: once v depends on c alone, a
// dependent that is no deferrable member waits on it, so c is actionable.
func TestEpicReady_KeepsAConditionWorkWaitsOn(t *testing.T) {
	g := deferrableGraph()
	g.Epics[2].DependsOn = []model.EpicDep{{ID: "c", Why: "c first"}}
	states := anyOfStates(g, "r")
	assert.Equal(t, []string{"c", "free"}, EpicReady(g, states, nil, false).Conditions)
}

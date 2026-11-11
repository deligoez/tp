package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// operatorGroupsGraph is one open milestone A and a closed B after it: x waits
// on one of the conditions c1 and c2, y on one of the work epics w1 and w2, z
// (a deferrable member of A's gate) on one of c1 and c3, v on one of c1 and
// the done c4, and u in the closed B on one of c2 and c3.
func operatorGroupsGraph() *model.EpicGraph {
	cond := func(id string) model.Epic {
		return model.Epic{ID: id, Title: id, Milestone: "A", Condition: true, Acceptance: "a key arrives", Root: true}
	}
	group := func(id, m string, members ...string) model.Epic {
		return model.Epic{ID: id, Title: id, Milestone: m, DependsOn: []model.EpicDep{{AnyOf: members, Why: id + " needs one of them"}}}
	}
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		cond("c1"), cond("c2"), cond("c3"), cond("c4"),
		{ID: "w1", Title: "w1", Milestone: "A", Root: true},
		{ID: "w2", Title: "w2", Milestone: "A", Root: true},
		group("x", "A", "c1", "c2"),
		group("y", "A", "w1", "w2"),
		group("z", "A", "c1", "c3"),
		group("v", "A", "c1", "c4"),
		group("u", "B", "c2", "c3"),
	}, Gates: map[string]model.EpicGate{"A": {Deferrable: []string{"z"}}}}
}

// TestDashboardPlan_OperatorGroupsAreTheConditionChoicesOfOpenMilestones is
// spec/1.8.0.md §7's any-of rows: an unsatisfied group whose every member is
// a condition, held by an undone work epic of an open milestone that is not a
// deferrable member, is one operator choice. A group of work epics, a group
// one done member satisfies, a deferrable member's group and a closed
// milestone's group are not.
func TestDashboardPlan_OperatorGroupsAreTheConditionChoicesOfOpenMilestones(t *testing.T) {
	t.Parallel()
	g := operatorGroupsGraph()
	plan := DashboardPlanOf(g, anyOfStates(g, "c4"), nil, defaultMatcher(t), nil)
	assert.Equal(t, []DashboardChoice{{Epic: "x", AnyOf: []string{"c1", "c2"}}}, plan.OperatorGroups)
}

// TestDashboardPlan_OperatorQueueCountsGroupsAndGrantableSlices: each
// operator choice and each slice an operator may grant a round is one
// decision in the queue, besides the conditions.
func TestDashboardPlan_OperatorQueueCountsGroupsAndGrantableSlices(t *testing.T) {
	t.Parallel()
	g := operatorGroupsGraph()
	slices := []EpicSliceState{
		{Dir: "a", EpicLoop: EpicLoop{Round: 3, State: LoopStateStale}, Grantable: true},
		{Dir: "b", EpicLoop: EpicLoop{Round: 2, State: LoopStateOpen}},
	}
	plan := DashboardPlanOf(g, anyOfStates(g, "c4"), nil, defaultMatcher(t), slices)
	assert.Equal(t, len(plan.OperatorConditions)+1+1, plan.Figures.OperatorQueue)
}

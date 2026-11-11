package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// figuresGraph is spec/1.8.0.md §18 row 7's condition half: one milestone
// holding a ready work epic w, a condition c blocked by w, a work epic h held
// by c, a condition free with every dependency done, and a done condition k.
func figuresGraph() *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "w", Title: "W", Milestone: "M", Root: true},
		{ID: "c", Title: "C", Milestone: "M", Condition: true, Acceptance: "x", DependsOn: []model.EpicDep{{ID: "w", Why: "w first"}}},
		{ID: "h", Title: "H", Milestone: "M", DependsOn: []model.EpicDep{{ID: "c", Why: "c first"}}},
		{ID: "free", Title: "Free", Milestone: "M", Condition: true, Acceptance: "x", Root: true},
		{ID: "k", Title: "K", Milestone: "M", Condition: true, Acceptance: "x", Root: true},
	}}
}

// TestEpicWorkFigures_CountNoCondition is §18 row 7, condition half: a
// condition is counted in neither ready nor blocked, nor held nor share done.
// A filter applied to only one figure counts c as blocked or k as done.
func TestEpicWorkFigures_CountNoCondition(t *testing.T) {
	g := figuresGraph()
	states := anyOfStates(g, "k")
	ready := EpicReady(g, states, nil, false)
	assert.Equal(t, EpicWorkFigures{Work: 2, Ready: 1, Blocked: 1, Held: 1, Done: 0, ShareDone: 0},
		WorkFigures(g, states, ready.Ready, "M"))
	assert.Equal(t, WorkFigures(g, states, ready.Ready, "M"), WorkFigures(g, states, ready.Ready, ""),
		"an empty milestone counts the whole graph")
}

// TestEpicWorkFigures_OnlyUndoneEpicIsACondition is §4's acceptance: a graph
// whose only undone epic is a condition shows zero ready and zero blocked
// epics, and the condition under the operator's conditions while actionable.
func TestEpicWorkFigures_OnlyUndoneEpicIsACondition(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{
		{ID: "w", Title: "W", Milestone: "M", Root: true},
		{ID: "c", Title: "C", Milestone: "M", Condition: true, Acceptance: "x", DependsOn: []model.EpicDep{{ID: "w", Why: "w first"}}},
	}}
	states := anyOfStates(g, "w")
	ready := EpicReady(g, states, nil, false)
	assert.Empty(t, ready.Ready)
	assert.Equal(t, []string{"c"}, ready.Conditions)
	assert.Equal(t, EpicWorkFigures{Work: 1, Done: 1, ShareDone: 1}, WorkFigures(g, states, ready.Ready, ""))
}

// TestIsWorkEpic: a condition is the operator's precondition, not work.
func TestIsWorkEpic(t *testing.T) {
	assert.True(t, IsWorkEpic(&model.Epic{ID: "w"}))
	assert.False(t, IsWorkEpic(&model.Epic{ID: "c", Condition: true}))
}

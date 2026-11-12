package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// defaultMatcher is the default placeholder pattern's matcher.
func defaultMatcher(t *testing.T) *PlaceholderMatcher {
	t.Helper()
	m, err := NewPlaceholderMatcher(DefaultThresholdPlaceholder)
	require.NoError(t, err)
	return m
}

// TestDashboardPlan_FiguresAndNowListsCountNoCondition is spec/1.8.0.md §4's
// KPI strip and now lists over figuresGraph: two work epics, w ready and h
// held by the condition c, the actionable condition free, and the done
// condition k. Conditions are in no figure and in no ready, waiting or
// most-waited-on list; free is the one operator decision; w is waited on by h
// through c, which is passed through and not counted. Two of three slices
// are open loops.
func TestDashboardPlan_FiguresAndNowListsCountNoCondition(t *testing.T) {
	t.Parallel()
	g := figuresGraph()
	states := anyOfStates(g, "k")
	slices := []EpicSliceState{
		{Dir: "a", EpicLoop: EpicLoop{Round: 2, State: LoopStateOpen}},
		{Dir: "b", EpicLoop: EpicLoop{Round: 3, State: LoopStateDone}},
		{Dir: "c", EpicLoop: EpicLoop{Round: 1, State: LoopStateStale}},
	}
	plan := DashboardPlanOf(g, states, nil, defaultMatcher(t), slices)

	critical := GraphCriticalPath(g, states)
	require.NotEmpty(t, critical)
	assert.Equal(t, DashboardFigures{
		Work: 2, Ready: 1, Blocked: 1, Held: 1, Done: 0, ShareDone: 0,
		CriticalPath: len(critical), OperatorQueue: 1, OpenLoops: 2, Throughput: []DashboardWeek{},
	}, plan.Figures)
	assert.Equal(t, []string{"w"}, plan.Ready)
	assert.Equal(t, []string{"free"}, plan.OperatorConditions)
	assert.Equal(t, []string{"h"}, plan.Waiting)
	assert.Equal(t, []string{"w"}, plan.MostWaitedOn)
}

// TestDashboardPlan_OperatorQueueCountsGatesToMeasureAndRewrite: a gate whose
// members are done and whose threshold is written waits on --met, and an
// unmet gate whose threshold is a placeholder waits on its rewrite; each is
// one decision in the queue.
func TestDashboardPlan_OperatorQueueCountsGatesToMeasureAndRewrite(t *testing.T) {
	t.Parallel()
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}, {"B"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "A", Root: true},
		{ID: "b", Title: "B", Milestone: "B", Root: true},
	}, Gates: map[string]model.EpicGate{"A": {Threshold: "p95 under 200ms"}, "B": {Threshold: "TBD"}}}
	plan := DashboardPlanOf(g, anyOfStates(g, "a"), nil, defaultMatcher(t), nil)
	assert.Equal(t, 2, plan.Figures.OperatorQueue)
	assert.Empty(t, plan.OperatorConditions)
}

// TestDashboardPlan_ThroughputCountsDoneWorkEpicsPerWeekAcrossMilestones:
// the graph-wide throughput sums every milestone's done work epics per ISO
// week, in week order; a done condition counts in none.
func TestDashboardPlan_ThroughputCountsDoneWorkEpicsPerWeekAcrossMilestones(t *testing.T) {
	t.Parallel()
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		{ID: "a1", Title: "A1", Milestone: "A", Root: true},
		{ID: "b1", Title: "B1", Milestone: "B", Root: true},
		{ID: "b2", Title: "B2", Milestone: "B", Root: true},
		{ID: "c", Title: "C", Milestone: "B", Condition: true, Acceptance: "x", Root: true},
	}}
	week45 := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	week46 := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	states := map[string]EpicState{
		"a1": {Status: EpicStatusDone, DoneAt: &week45},
		"b1": {Status: EpicStatusDone, DoneAt: &week45},
		"b2": {Status: EpicStatusDone, DoneAt: &week46},
		"c":  {Status: EpicStatusDone, DoneAt: &week46},
	}
	plan := DashboardPlanOf(g, states, nil, defaultMatcher(t), nil)
	assert.Equal(t, []DashboardWeek{{Week: "2026-W45", Done: 2}, {Week: "2026-W46", Done: 1}}, plan.Figures.Throughput)
	assert.InDelta(t, 1.0, plan.Figures.ShareDone, 0)
}

// TestDashboardSummary_CarriesThePlanWithAGraph: in a repository whose epic b
// is bound to a task file with an open task, the summary's plan reads the
// bound file: b is not ready, a waits on it and is the one blocked epic, b is
// the most waited on, and two of four work epics are done.
func TestDashboardSummary_CarriesThePlanWithAGraph(t *testing.T) {
	t.Parallel()
	root := boundFilesRepo(t)
	tpDir := filepath.Join(root, ".tp")
	require.NoError(t, os.MkdirAll(tpDir, 0o755))
	writeStatusFile(t, root, "b", model.StatusOpen)

	sum, err := DashboardSummaryOf(tpDir, statusChain())
	require.NoError(t, err)
	assert.Equal(t, DashboardFigures{Work: 4, Blocked: 1, Done: 2, ShareDone: 0.5, CriticalPath: 2,
		Throughput: []DashboardWeek{{Week: "2026-W02", Done: 2}}}, sum.Plan.Figures)
	assert.Empty(t, sum.Plan.Ready)
	assert.Equal(t, []string{"a"}, sum.Plan.Waiting)
	assert.Equal(t, []string{"b"}, sum.Plan.MostWaitedOn)
}

// TestDashboardPlan_MostWaitedOnRanksDependentsBeforeChainLength: a has
// three direct dependents and a chain of one, b two dependents in a chain of
// two, and y1 one; the list ranks by dependents first, so a leads though b's
// chain is longer, whichever of a and b the graph lists first.
func TestDashboardPlan_MostWaitedOnRanksDependentsBeforeChainLength(t *testing.T) {
	t.Parallel()
	dep := func(id string) []model.EpicDep { return []model.EpicDep{{ID: id, Why: "w"}} }
	a := model.Epic{ID: "a", Title: "A", Milestone: "M", Root: true}
	b := model.Epic{ID: "b", Title: "B", Milestone: "M", Root: true}
	for _, first := range [][]model.Epic{{b, a}, {a, b}} {
		g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: append([]model.Epic{first[0], first[1]},
			model.Epic{ID: "x1", Title: "X1", Milestone: "M", DependsOn: dep("a")},
			model.Epic{ID: "x2", Title: "X2", Milestone: "M", DependsOn: dep("a")},
			model.Epic{ID: "x3", Title: "X3", Milestone: "M", DependsOn: dep("a")},
			model.Epic{ID: "y1", Title: "Y1", Milestone: "M", DependsOn: dep("b")},
			model.Epic{ID: "y2", Title: "Y2", Milestone: "M", DependsOn: dep("y1")},
		)}
		plan := DashboardPlanOf(g, anyOfStates(g), nil, defaultMatcher(t), nil)
		assert.Equal(t, []string{"a", "b", "y1"}, plan.MostWaitedOn, "graph order %s, %s", first[0].ID, first[1].ID)
	}
}

// TestDashboardSummary_ThePlanReadsTheProjectsPlaceholderPattern: the
// project's pattern ^draft: makes gate A's "draft: later" a placeholder, one
// rewrite in the operator queue though a's work is not done; under the
// default pattern the text reads as written and the queue is empty.
func TestDashboardSummary_ThePlanReadsTheProjectsPlaceholderPattern(t *testing.T) {
	t.Parallel()
	root := boundFilesRepo(t)
	tpDir := filepath.Join(root, ".tp")
	require.NoError(t, os.MkdirAll(tpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "config.json"), []byte(`{"epic":{"threshold_placeholder":"^draft:"}}`), 0o600))
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "A", Root: true},
	}, Gates: map[string]model.EpicGate{"A": {Threshold: "draft: later"}}}

	sum, err := DashboardSummaryOf(tpDir, g)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Plan.Figures.OperatorQueue)
}

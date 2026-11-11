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

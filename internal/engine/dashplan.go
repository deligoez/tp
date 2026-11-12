package engine

import (
	"cmp"
	"slices"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// DashboardPlan is the Epics tab's KPI figures and now lists (spec/1.8.0.md
// §4), derived once so the page counts and ranks nothing. Every list holds
// epic ids.
type DashboardPlan struct {
	Figures DashboardFigures `json:"figures"`
	// Ready is tp epic ready's list, in its order.
	Ready []string `json:"ready"`
	// OperatorConditions is tp epic ready's actionable conditions, which
	// leave out a deferrable condition and one only deferrable work waits on
	// (ConditionsNotActionable).
	OperatorConditions []string `json:"operator_conditions"`
	// OperatorGroups is DashboardOperatorGroups: the any-of groups of
	// conditions the operator settles by choosing one.
	OperatorGroups []DashboardChoice `json:"operator_groups"`
	// Waiting is DashboardWaiting over the whole graph.
	Waiting []string `json:"waiting"`
	// MostWaitedOn holds the not-done work epics other work epics depend on,
	// most dependents first, then longest chain, then graph order.
	MostWaitedOn []string `json:"most_waited_on"`
}

// DashboardFigures is the KPI strip: WorkFigures over the whole graph, the
// length of tp epic status's top-level critical path, the operator queue's
// length (the operator conditions, the any-of groups to choose in, the gates
// whose written threshold awaits --met, the placeholder thresholds to rewrite
// and the slices that may take --grant-round), the epic review slices
// whose loop is not done, and the graph-wide throughput per ISO week.
type DashboardFigures struct {
	Work          int             `json:"work"`
	Ready         int             `json:"ready"`
	Blocked       int             `json:"blocked"`
	Held          int             `json:"held"`
	Done          int             `json:"done"`
	ShareDone     float64         `json:"share_done"`
	CriticalPath  int             `json:"critical_path"`
	OperatorQueue int             `json:"operator_queue"`
	OpenLoops     int             `json:"open_loops"`
	Throughput    []DashboardWeek `json:"throughput"`
}

// DashboardWeek is one ISO week's count of work epics done in it.
type DashboardWeek struct {
	Week string `json:"week"`
	Done int    `json:"done"`
}

// emptyDashboardPlan is the plan of a repository with no graph.
func emptyDashboardPlan() DashboardPlan {
	return DashboardPlan{Figures: DashboardFigures{Throughput: []DashboardWeek{}}, Ready: []string{},
		OperatorConditions: []string{}, OperatorGroups: []DashboardChoice{}, Waiting: []string{}, MostWaitedOn: []string{}}
}

// DashboardPlanOf derives the plan from g, its derived states, its bound
// files, the project's placeholder matcher and tp epic export's slices.
func DashboardPlanOf(g *model.EpicGraph, states map[string]EpicState, bound map[string][]string, matcher *PlaceholderMatcher, reviewSlices []EpicSliceState) DashboardPlan {
	plan := emptyDashboardPlan()
	r := EpicReady(g, states, bound, false)
	f := WorkFigures(g, states, r.Ready, "")
	plan.Ready = append(plan.Ready, r.Ready...)
	plan.OperatorConditions = append(plan.OperatorConditions, r.Conditions...)
	plan.OperatorGroups = DashboardOperatorGroups(g, states)
	plan.Waiting = DashboardWaiting(g, states, "")
	plan.MostWaitedOn = mostWaitedOn(g, states)
	queue := len(plan.OperatorConditions) + len(plan.OperatorGroups) + len(WithoutPlaceholderGates(g, r.Gates, matcher)) +
		len(ThresholdRewrites(g, matcher)) + grantableSlices(reviewSlices)
	plan.Figures = DashboardFigures{
		Work: f.Work, Ready: f.Ready, Blocked: f.Blocked, Held: f.Held, Done: f.Done, ShareDone: f.ShareDone,
		CriticalPath:  len(GraphCriticalPath(g, states)),
		OperatorQueue: queue,
		OpenLoops:     openLoops(reviewSlices),
		Throughput:    graphThroughput(g, states),
	}
	return plan
}

// mostWaitedOn ranks the not-done work epics with at least one dependent.
func mostWaitedOn(g *model.EpicGraph, states map[string]EpicState) []string {
	ids := []string{}
	for i := range g.Epics {
		if IsWorkEpic(&g.Epics[i]) {
			ids = append(ids, g.Epics[i].ID)
		}
	}
	down := EpicDownstreams(g, states, MilestoneGates(g, states), ids)
	out := []string{}
	for _, id := range ids {
		if d, ok := down[id]; ok && d.Dependents > 0 {
			out = append(out, id)
		}
	}
	slices.SortStableFunc(out, func(a, b string) int {
		if c := cmp.Compare(down[b].Dependents, down[a].Dependents); c != 0 {
			return c
		}
		return cmp.Compare(down[b].LongestChain, down[a].LongestChain)
	})
	return out
}

// openLoops counts the slices whose loop is not done.
func openLoops(reviewSlices []EpicSliceState) int {
	n := 0
	for i := range reviewSlices {
		if reviewSlices[i].State != LoopStateDone {
			n++
		}
	}
	return n
}

// graphThroughput sums EpicThroughput over every milestone g's epics name.
func graphThroughput(g *model.EpicGraph, states map[string]EpicState) []DashboardWeek {
	counts := map[string]int{}
	seen := map[string]bool{}
	for i := range g.Epics {
		m := g.Epics[i].Milestone
		if seen[m] {
			continue
		}
		seen[m] = true
		for _, w := range EpicThroughput(g, states, m) {
			counts[w.Week] += w.Done
		}
	}
	out := make([]DashboardWeek, 0, len(counts))
	for week, n := range counts {
		out = append(out, DashboardWeek{Week: week, Done: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Week < out[j].Week })
	return out
}

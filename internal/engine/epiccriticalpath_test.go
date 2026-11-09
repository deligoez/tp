package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// criticalPathOf derives g's states with no bound file and returns
// milestone A's critical path under estimates.
func criticalPathOf(t *testing.T, g *model.EpicGraph, estimates map[string]int) EpicCriticalPath {
	t.Helper()
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)
	return CriticalPath(g, states, estimates, "A")
}

// TestCriticalPath_DoneEpicCutsTheChain is spec/1.5.0.md §10 row 12 with
// spec/1.5.0-corrections.md C5: on the chain a→b→c→d with b done and A's gate
// on a, the path is [a]. A path that counted the done b would be [d c b a], and
// one that crossed it [d c a] or [d c].
func TestCriticalPath_DoneEpicCutsTheChain(t *testing.T) {
	g := epicChainGraph()
	g.Epics[1].Done = &model.EpicDone{At: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), Evidence: "shipped"}
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}}}

	path := criticalPathOf(t, g, nil)

	assert.Equal(t, []string{"a"}, path.ByCount)

	g.Epics[1].Done = nil
	assert.Equal(t, []string{"d", "c", "b", "a"}, criticalPathOf(t, g, nil).ByCount,
		"with b not done the chain runs from d to the member")
}

// estimateGraph is a member m of milestone A fed by two chains: x2→x, two
// light epics, and y, one heavy epic of milestone B. The longest chain by
// count runs through x2 and x; the heaviest by estimate through y.
func estimateGraph() *model.EpicGraph {
	dep := func(ids ...string) []model.EpicDep {
		deps := make([]model.EpicDep, 0, len(ids))
		for _, id := range ids {
			deps = append(deps, model.EpicDep{ID: id, Why: "needs it"})
		}
		return deps
	}
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}, {"B"}}, Epics: []model.Epic{
		{ID: "m", Title: "M", Milestone: "A", DependsOn: dep("x", "y")},
		{ID: "x", Title: "X", Milestone: "A", DependsOn: dep("x2")},
		{ID: "x2", Title: "X2", Milestone: "A", Root: true},
		{ID: "y", Title: "Y", Milestone: "B", Root: true},
	}, Gates: map[string]model.EpicGate{"A": {Members: []string{"m"}}}}
}

// TestCriticalPath_EstimateChainChosenApartFromCount is C2: the estimate
// chain is the heaviest by summed estimate among the chains ending at a gate
// member, chosen independently of the longest by count. An estimate chain
// that summed the count path would report [x2 x m] at 7 minutes.
func TestCriticalPath_EstimateChainChosenApartFromCount(t *testing.T) {
	estimates := map[string]int{"m": 5, "x": 1, "x2": 1, "y": 30}

	path := criticalPathOf(t, estimateGraph(), estimates)

	assert.Equal(t, []string{"x2", "x", "m"}, path.ByCount)
	assert.Equal(t, []string{"y", "m"}, path.ByEstimate)
	assert.Equal(t, 35, path.EstimateMinutes)
}

// TestCriticalPath_EstimateChainOmittedWhenAnEpicHasNone: when one epic on
// any chain ending at a gate member has no estimate, no estimate chain is
// reported, even when that epic is not on the heaviest chain. A check that
// read only the heaviest chain's epics would report [y m].
func TestCriticalPath_EstimateChainOmittedWhenAnEpicHasNone(t *testing.T) {
	estimates := map[string]int{"m": 5, "x": 1, "y": 30}

	path := criticalPathOf(t, estimateGraph(), estimates)

	assert.Equal(t, []string{"x2", "x", "m"}, path.ByCount)
	assert.Nil(t, path.ByEstimate)
	assert.Zero(t, path.EstimateMinutes)
}

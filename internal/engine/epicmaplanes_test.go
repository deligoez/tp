package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// lanesGraph is tracks and opens_after as given, with no epics: the lanes
// read the milestones alone.
func lanesGraph(tracks [][]string, opensAfter map[string][]string) *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: tracks, OpensAfter: opensAfter}
}

// TestEpicMapLanes_EveryMilestoneFallsIntoOneLaneByPrecedence is §18 row 8:
// T1 and T2 carry an explicit [] (T2 in the middle of a track, where a missing
// entry would extend the N chain), N1 and B1 are track firsts with no entry,
// N1-N3 and B1-B3 are two equally long chains of which N's is listed first
// though B sorts first by name, and V opens after N1 (a parallel vertical
// listed before the B chain).
func TestEpicMapLanes_EveryMilestoneFallsIntoOneLaneByPrecedence(t *testing.T) {
	g := lanesGraph(
		[][]string{{"N1", "N2", "N3", "T2"}, {"V"}, {"B1", "B2", "B3"}, {"T1"}},
		map[string][]string{"T1": {}, "T2": {}, "V": {"N1"}},
	)
	got := EpicMapLanes(g)
	assert.Equal(t, []MapLane{
		{Kind: MapLaneMain, Milestones: []string{"N1", "N2", "N3"}},
		{Kind: MapLaneParallel, Milestones: []string{"V"}},
		{Kind: MapLaneParallel, Milestones: []string{"B1", "B2", "B3"}},
		{Kind: MapLaneTriggered, Milestones: []string{"T2", "T1"}},
	}, got.Lanes)
	assert.Equal(t, map[string]int{
		"N1": 0, "N2": 1, "N3": 2, "T2": 0, "V": 1, "B1": 0, "B2": 1, "B3": 2, "T1": 0,
	}, got.Columns)
}

// TestEpicMapLanes_EqualChainsFromOneMilestoneTieOnTheFirstDifference: three
// chains of three start at S; Z's second milestone is listed first, so it is
// the main lane, and the C and B chains are two parallel lanes in file order.
func TestEpicMapLanes_EqualChainsFromOneMilestoneTieOnTheFirstDifference(t *testing.T) {
	g := lanesGraph(
		[][]string{{"S", "Z1", "Z2"}, {"C1", "C2"}, {"B1", "B2"}},
		map[string][]string{"C1": {"S"}, "B1": {"S"}},
	)
	got := EpicMapLanes(g)
	assert.Equal(t, []MapLane{
		{Kind: MapLaneMain, Milestones: []string{"S", "Z1", "Z2"}},
		{Kind: MapLaneParallel, Milestones: []string{"C1", "C2"}},
		{Kind: MapLaneParallel, Milestones: []string{"B1", "B2"}},
	}, got.Lanes)
	assert.Equal(t, map[string]int{"S": 0, "Z1": 1, "Z2": 2, "C1": 1, "C2": 2, "B1": 1, "B2": 2}, got.Columns)
}

// TestEpicMapLanes_ACycleStillPlacesEveryMilestoneOnce: a graph lint refuses
// still draws, each milestone in one lane.
func TestEpicMapLanes_ACycleStillPlacesEveryMilestoneOnce(t *testing.T) {
	g := lanesGraph([][]string{{"A", "B"}}, map[string][]string{"A": {"B"}})
	got := EpicMapLanes(g)
	placed := []string{}
	for _, lane := range got.Lanes {
		placed = append(placed, lane.Milestones...)
	}
	assert.ElementsMatch(t, []string{"A", "B"}, placed)
	assert.Len(t, got.Columns, 2)
}

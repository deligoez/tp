package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// gatesOf derives g's taskless epics and returns every milestone's gate.
func gatesOf(t *testing.T, g *model.EpicGraph) map[string]MilestoneGate {
	t.Helper()
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)
	return MilestoneGates(g, states)
}

func epicDoneRecord() *model.EpicDone {
	return &model.EpicDone{At: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), Evidence: "shipped"}
}

// TestMilestoneGates_ImplicitGateHoldsTheNextMilestone is spec/1.5.0.md §10
// row 7: milestones [[A, B, C]], an epic in each, a gate named only on B. B
// opens once A's epic is done, though gates names no entry for A, and C opens
// only once B's gate is green. A second track opens on its own. Counting a
// milestone gates does not name as green would open B at once.
func TestMilestoneGates_ImplicitGateHoldsTheNextMilestone(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B", "C"}, {"L1"}}, Epics: []model.Epic{
		{ID: "x", Title: "X", Milestone: "A", Root: true},
		{ID: "y", Title: "Y", Milestone: "B", Root: true},
		{ID: "z", Title: "Z", Milestone: "C", Root: true},
		{ID: "l", Title: "L", Milestone: "L1", Root: true},
	}, Gates: map[string]model.EpicGate{"B": {Members: []string{"y"}}}}

	gates := gatesOf(t, g)
	assert.Equal(t, MilestoneGate{Green: false, Open: true}, gates["A"], "the first milestone of a track is always open")
	assert.Equal(t, MilestoneGate{Green: false, Open: false}, gates["B"], "A's implicit gate is red while x is not done")
	assert.Equal(t, MilestoneGate{Green: false, Open: false}, gates["C"])
	assert.Equal(t, MilestoneGate{Green: false, Open: true}, gates["L1"], "tracks do not wait on each other")

	g.Epics[0].Done = epicDoneRecord()
	gates = gatesOf(t, g)
	assert.True(t, gates["A"].Green)
	assert.True(t, gates["B"].Open, "B opens once A's epics are done")
	assert.False(t, gates["C"].Open, "C waits on B's gate")

	g.Epics[1].Done = epicDoneRecord()
	gates = gatesOf(t, g)
	assert.True(t, gates["C"].Open, "C opens once B's gate is green")
}

// TestMilestoneGates_ThresholdNeedsAMetRecord: a gate carrying a threshold is
// red with every member done until a met record exists.
func TestMilestoneGates_ThresholdNeedsAMetRecord(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		{ID: "x", Title: "X", Milestone: "A", Root: true, Done: epicDoneRecord()},
		{ID: "y", Title: "Y", Milestone: "B", Root: true},
	}, Gates: map[string]model.EpicGate{"A": {Threshold: "p95 under 200ms"}}}

	gates := gatesOf(t, g)
	assert.False(t, gates["A"].Green, "a threshold not yet met holds the gate")
	assert.False(t, gates["B"].Open)

	gate := g.Gates["A"]
	gate.Met = &model.GateMet{At: time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC), Evidence: "dashboard", Threshold: gate.Threshold}
	g.Gates["A"] = gate
	gates = gatesOf(t, g)
	assert.True(t, gates["A"].Green)
	assert.True(t, gates["B"].Open)
}

// TestMilestoneGates_MemberFallingBackTurnsTheGateRed: members are read live,
// so a member that is no longer done turns a met gate red again while its met
// record stands.
func TestMilestoneGates_MemberFallingBackTurnsTheGateRed(t *testing.T) {
	met := &model.GateMet{At: time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC), Evidence: "dashboard", Threshold: "p95 under 200ms"}
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A", "B"}}, Epics: []model.Epic{
		{ID: "x", Title: "X", Milestone: "A", Root: true, Done: epicDoneRecord()},
		{ID: "y", Title: "Y", Milestone: "B", Root: true},
	}, Gates: map[string]model.EpicGate{"A": {Threshold: "p95 under 200ms", Met: met}}}
	require.True(t, gatesOf(t, g)["A"].Green)

	g.Epics[0].Done = nil
	gates := gatesOf(t, g)
	assert.False(t, gates["A"].Green, "a member that falls back turns the gate red")
	assert.False(t, gates["B"].Open)
	assert.NotNil(t, g.Gates["A"].Met, "the met record stands")
}

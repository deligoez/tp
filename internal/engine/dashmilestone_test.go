package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// milestoneStateGraph is spec/1.8.0.md §6's milestone state: M1's gate has
// three members a, b, c and a fourth, deferrable member d; M2 follows M1 in
// its track, and N, on a track of its own, opens after M1 by opens_after.
func milestoneStateGraph(threshold string) *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{{"M1", "M2"}, {"N"}},
		OpensAfter: map[string][]string{"N": {"M1"}},
		Gates: map[string]model.EpicGate{"M1": {
			Members: []string{"a", "b", "c"}, Deferrable: []string{"d"}, Threshold: threshold,
		}},
		Epics: []model.Epic{
			{ID: "a", Title: "A", Milestone: "M1", Root: true},
			{ID: "b", Title: "B", Milestone: "M1", Root: true},
			{ID: "c", Title: "C", Milestone: "M1", Root: true},
			{ID: "d", Title: "D", Milestone: "M1", Root: true},
			{ID: "e", Title: "E", Milestone: "M2", Root: true},
			{ID: "n", Title: "N", Milestone: "N", Root: true},
		}}
}

// milestoneStateOf returns the entry for m.
func milestoneStateOf(t *testing.T, states []MilestoneState, m string) MilestoneState {
	t.Helper()
	for _, s := range states {
		if s.Milestone == m {
			return s
		}
	}
	require.Failf(t, "no milestone state", "%s in %+v", m, states)
	return MilestoneState{}
}

// TestMilestoneStates_AClosedMilestoneNamesWhatItWaitsOn: M1's gate is unmet,
// so N, whose opens_after names M1, reads closed and names M1, as does M2,
// which follows it in its track; M1 opens after nothing.
func TestMilestoneStates_AClosedMilestoneNamesWhatItWaitsOn(t *testing.T) {
	g := milestoneStateGraph("")
	m, err := NewPlaceholderMatcher("")
	require.NoError(t, err)
	states := MilestoneStates(g, anyOfStates(g, "a", "b", "d"), m)
	assert.Equal(t, []string{"M1", "M2", "N"}, []string{states[0].Milestone, states[1].Milestone, states[2].Milestone},
		"track order")
	first := milestoneStateOf(t, states, "M1")
	assert.True(t, first.Open)
	assert.Equal(t, []string{}, first.WaitingOn)
	for _, name := range []string{"M2", "N"} {
		s := milestoneStateOf(t, states, name)
		assert.False(t, s.Open, name)
		assert.Equal(t, []string{"M1"}, s.WaitingOn, name)
	}

	open := MilestoneStates(g, anyOfStates(g, "a", "b", "c"), m)
	assert.True(t, milestoneStateOf(t, open, "N").Open, "a green M1 opens N, the deferrable d undone")
}

// TestMilestoneStates_CountsMembersDoneLeavingDeferrableOut: with a, b done,
// c not and the deferrable d done, the gate reads 2 of 3.
func TestMilestoneStates_CountsMembersDoneLeavingDeferrableOut(t *testing.T) {
	g := milestoneStateGraph("")
	m, err := NewPlaceholderMatcher("")
	require.NoError(t, err)
	s := milestoneStateOf(t, MilestoneStates(g, anyOfStates(g, "a", "b", "d"), m), "M1")
	assert.Equal(t, 2, s.MembersDone)
	assert.Equal(t, 3, s.Members)

	implicit := milestoneStateGraph("")
	implicit.Gates["M1"] = model.EpicGate{Deferrable: []string{"d"}}
	s = milestoneStateOf(t, MilestoneStates(implicit, anyOfStates(implicit, "a", "b", "d"), m), "M1")
	assert.Equal(t, [2]int{2, 3}, [2]int{s.MembersDone, s.Members}, "the implicit members leave d out too")
}

// TestMilestoneStates_ThresholdState: a threshold reads written, absent,
// placeholder by the §8 verdict, or met once recorded, whatever its text.
func TestMilestoneStates_ThresholdState(t *testing.T) {
	m, err := NewPlaceholderMatcher("")
	require.NoError(t, err)
	for threshold, want := range map[string]string{
		"p95 under 200ms": ThresholdStateWritten,
		"":                ThresholdStateAbsent,
		"  ":              ThresholdStateAbsent,
		"[threshold]":     ThresholdStatePlaceholder,
		"TBD":             ThresholdStatePlaceholder,
	} {
		g := milestoneStateGraph(threshold)
		s := milestoneStateOf(t, MilestoneStates(g, anyOfStates(g), m), "M1")
		assert.Equal(t, want, s.Threshold, "%q", threshold)
	}
	for _, threshold := range []string{"p95 under 200ms", "TBD"} {
		g := milestoneStateGraph(threshold)
		gate := g.Gates["M1"]
		gate.Met = &model.GateMet{Evidence: "measured", Threshold: threshold}
		g.Gates["M1"] = gate
		s := milestoneStateOf(t, MilestoneStates(g, anyOfStates(g), m), "M1")
		assert.Equal(t, ThresholdStateMet, s.Threshold, "%q met", threshold)
	}
	draft, err := NewPlaceholderMatcher("^draft:")
	require.NoError(t, err)
	g := milestoneStateGraph("TBD")
	assert.Equal(t, ThresholdStateWritten, milestoneStateOf(t, MilestoneStates(g, anyOfStates(g), draft), "M1").Threshold,
		"the project pattern gives the verdict")
	assert.Equal(t, ThresholdStateAbsent, milestoneStateOf(t, MilestoneStates(g, anyOfStates(g), draft), "M2").Threshold,
		"a milestone with no gate entry has no threshold")
}

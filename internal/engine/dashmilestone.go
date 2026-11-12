package engine

import (
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// Threshold states of a milestone's gate (spec/1.8.0.md §6). A met record
// wins over the text; an empty or blank threshold is absent; otherwise the
// §8 placeholder verdict tells a placeholder from a written one.
const (
	ThresholdStateWritten     = "written"
	ThresholdStateAbsent      = "absent"
	ThresholdStatePlaceholder = "placeholder"
	ThresholdStateMet         = "met"
)

// MilestoneState is one milestone's entry beside the lifecycle band
// (spec/1.8.0.md §6): open or closed by its opens_after, with the milestones
// it waits on; its gate's members done out of members, deferrable members
// left out of both counts; and its threshold state.
type MilestoneState struct {
	Milestone   string   `json:"milestone"`
	Open        bool     `json:"open"`
	WaitingOn   []string `json:"waiting_on"`
	MembersDone int      `json:"members_done"`
	Members     int      `json:"members"`
	Threshold   string   `json:"threshold"`
}

// MilestoneStates returns the state of every milestone a track lists, in
// track order, each once. states is DeriveEpicStates' result and matcher
// gives the placeholder verdict.
func MilestoneStates(g *model.EpicGraph, states map[string]EpicState, matcher *PlaceholderMatcher) []MilestoneState {
	gates := MilestoneGates(g, states)
	after := EffectiveOpensAfter(g)
	out := []MilestoneState{}
	seen := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			if seen[m] {
				continue
			}
			seen[m] = true
			done, members := gateMembersCount(g, m, states)
			gate := g.Gates[m]
			out = append(out, MilestoneState{
				Milestone:   m,
				Open:        gates[m].Open,
				WaitingOn:   MilestoneWaitingOn(after[m], gates),
				MembersDone: done,
				Members:     members,
				Threshold:   thresholdState(&gate, matcher),
			})
		}
	}
	return out
}

// gateMembersCount counts milestone's gate members and those done, leaving
// out a deferrable epic even where an explicit members list names it.
func gateMembersCount(g *model.EpicGraph, milestone string, states map[string]EpicState) (done, members int) {
	ids, deferrable := GateMembers(g, milestone)
	for _, id := range ids {
		if slices.Contains(deferrable, id) {
			continue
		}
		members++
		if states[id].Status == EpicStatusDone {
			done++
		}
	}
	return done, members
}

// thresholdState is the gate's threshold state.
func thresholdState(gate *model.EpicGate, matcher *PlaceholderMatcher) string {
	switch {
	case gate.Met != nil:
		return ThresholdStateMet
	case strings.TrimSpace(gate.Threshold) == "":
		return ThresholdStateAbsent
	case matcher.IsPlaceholder(gate.Threshold):
		return ThresholdStatePlaceholder
	default:
		return ThresholdStateWritten
	}
}

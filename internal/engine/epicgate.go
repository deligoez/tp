package engine

import (
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// GateMembers returns milestone's effective gate (spec/1.5.0.md §4): the
// members that hold it and the deferrable epics that never do. An explicit
// members list is the gate's members as written. When the gate names no
// members, or gates names no entry for the milestone, the members are the
// milestone's epics, in graph order, that are neither conditions nor listed
// deferrable. Both slices are fresh copies a caller may modify.
func GateMembers(g *model.EpicGraph, milestone string) (members, deferrable []string) {
	gate := g.Gates[milestone]
	deferrable = slices.Clone(gate.Deferrable)
	if len(gate.Members) > 0 {
		return slices.Clone(gate.Members), deferrable
	}
	members = []string{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.Milestone == milestone && !e.Condition && !slices.Contains(deferrable, e.ID) {
			members = append(members, e.ID)
		}
	}
	return members, deferrable
}

// MilestoneGate is one milestone's verdict: whether its gate is green and
// whether the milestone is open.
type MilestoneGate struct {
	Green bool
	Open  bool
}

// MilestoneGates returns the verdict of every milestone a track lists, so
// ready, status and the critical path read one answer (spec/1.5.0.md §4).
// states is DeriveEpicStates' result. A milestone is open when every earlier
// milestone of its track has a green gate: the first of a track is always
// open, and tracks do not wait on each other. A milestone gates names no entry
// for still has its default gate. A milestone listed in no track is absent.
func MilestoneGates(g *model.EpicGraph, states map[string]EpicState) map[string]MilestoneGate {
	out := map[string]MilestoneGate{}
	for _, track := range g.Milestones {
		open := true
		for _, m := range track {
			green := GateGreen(g, m, states)
			out[m] = MilestoneGate{Green: green, Open: open}
			open = open && green
		}
	}
	return out
}

// GateGreen reports whether milestone's gate is green: every member done
// and, when the gate has a threshold, a met record. Members are read live
// from states, so one that falls back turns the gate red while its met record
// stands.
func GateGreen(g *model.EpicGraph, milestone string, states map[string]EpicState) bool {
	gate := g.Gates[milestone]
	if gate.Threshold != "" && gate.Met == nil {
		return false
	}
	members, _ := GateMembers(g, milestone)
	for _, id := range members {
		if states[id].Status != EpicStatusDone {
			return false
		}
	}
	return true
}

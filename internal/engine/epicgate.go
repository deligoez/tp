package engine

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
// ready, status, the export and the critical path read one answer
// (spec/1.5.0.md §4, spec/1.6.0.md §1). states is DeriveEpicStates' result. A
// milestone is open when every milestone it opens after (EffectiveOpensAfter)
// is open and has a green gate: one that opens after none is always open. A
// milestone on an opens_after cycle, or opening after a milestone no track
// lists, both lint errors, stays closed. A milestone gates names no entry for
// still has its default gate. A milestone listed in no track is absent.
func MilestoneGates(g *model.EpicGraph, states map[string]EpicState) map[string]MilestoneGate {
	o := milestoneOpening{after: EffectiveOpensAfter(g), green: map[string]bool{}, open: map[string]bool{}, visiting: map[string]bool{}}
	for m := range o.after {
		o.green[m] = GateGreen(g, m, states)
	}
	out := make(map[string]MilestoneGate, len(o.after))
	for m := range o.after {
		out[m] = MilestoneGate{Green: o.green[m], Open: o.isOpen(m)}
	}
	return out
}

// milestoneOpening memoizes which milestones are open.
type milestoneOpening struct {
	after    map[string][]string
	green    map[string]bool
	open     map[string]bool
	visiting map[string]bool
}

// isOpen reports whether m is open; a milestone met again while its own
// answer is pending closes the cycle, which keeps every milestone on it closed.
func (o *milestoneOpening) isOpen(m string) bool {
	if open, ok := o.open[m]; ok {
		return open
	}
	if o.visiting[m] {
		return false
	}
	o.visiting[m] = true
	open := true
	for _, x := range o.after[m] {
		if _, tracked := o.after[x]; !tracked || !o.green[x] || !o.isOpen(x) {
			open = false
			break
		}
	}
	o.visiting[m] = false
	o.open[m] = open
	return open
}

// EffectiveOpensAfter returns, for every milestone a track lists, the
// milestones it opens after (spec/1.6.0.md §1): its opens_after entry as
// written, else the previous milestone of its track, else none for a track's
// first. A milestone named in two tracks takes its first place. Every list is
// a fresh, non-nil copy.
func EffectiveOpensAfter(g *model.EpicGraph) map[string][]string {
	out := map[string][]string{}
	for _, track := range g.Milestones {
		for i, m := range track {
			if _, seen := out[m]; seen {
				continue
			}
			switch after, ok := g.OpensAfter[m]; {
			case ok:
				out[m] = append([]string{}, after...)
			case i == 0:
				out[m] = []string{}
			default:
				out[m] = []string{track[i-1]}
			}
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
	return gateMembersDone(g, milestone, states)
}

// gateMembersDone reports whether every member of milestone's gate is done.
func gateMembersDone(g *model.EpicGraph, milestone string, states map[string]EpicState) bool {
	members, _ := GateMembers(g, milestone)
	for _, id := range members {
		if states[id].Status != EpicStatusDone {
			return false
		}
	}
	return true
}

// GateMetRefusedError reports a met record refused on a milestone's gate
// (spec/1.5.0.md §4); nothing was written.
type GateMetRefusedError struct {
	Milestone string
	Reason    string
}

func (e *GateMetRefusedError) Error() string {
	return fmt.Sprintf("the gate of milestone %q cannot record met: %s", e.Milestone, e.Reason)
}

// RecordGateMet writes milestone's met record at at, copying the threshold it
// is judged against (spec/1.5.0.md §4). It returns an
// *EpicMilestoneNotFoundError for a milestone no track lists, and a
// *GateMetRefusedError while the threshold is empty, since a threshold is
// written before the measurement, or while a member is not done, since it is
// measured after the work. states is DeriveEpicStates' result.
func RecordGateMet(g *model.EpicGraph, milestone, evidence string, states map[string]EpicState, at time.Time) error {
	if _, tracked := MilestoneGates(g, states)[milestone]; !tracked {
		return &EpicMilestoneNotFoundError{Name: milestone}
	}
	gate := g.Gates[milestone]
	if gate.Threshold == "" {
		return &GateMetRefusedError{Milestone: milestone, Reason: "the gate has no threshold"}
	}
	members, _ := GateMembers(g, milestone)
	var notDone []string
	for _, id := range members {
		if states[id].Status != EpicStatusDone {
			notDone = append(notDone, fmt.Sprintf("%q", id))
		}
	}
	if len(notDone) > 0 {
		return &GateMetRefusedError{Milestone: milestone, Reason: "members not done: " + strings.Join(notDone, ", ")}
	}
	gate.Met = &model.GateMet{At: at.UTC(), Evidence: evidence, Threshold: gate.Threshold}
	g.Gates[milestone] = gate
	return nil
}

// MilestoneWaitingOn returns the milestones of after, in its order, that are
// not open or whose gate is not green: what still holds a milestone opening
// after them (spec/1.6.0.md §1), empty once it is open. gates is
// MilestoneGates' result; never nil.
func MilestoneWaitingOn(after []string, gates map[string]MilestoneGate) []string {
	out := []string{}
	for _, m := range after {
		if v := gates[m]; !v.Open || !v.Green {
			out = append(out, m)
		}
	}
	return out
}

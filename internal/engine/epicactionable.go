package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// Reasons a condition is not actionable (spec/1.6.0.md F3), in the fixed
// order ConditionsNotActionable lists them.
const (
	// NotActionableMilestone: the condition's milestone is not open.
	NotActionableMilestone = "milestone"
	// NotActionableDependencies: a direct dependency of its own is not done.
	NotActionableDependencies = "dependencies"
	// NotActionableDependents: no open milestone's gate holds it, something
	// depends on it, and no dependent that can move waits on conditions alone.
	NotActionableDependents = "dependents"
)

// ConditionsNotActionable returns, for every condition of g that is not done,
// the reasons it is not actionable, in the order NotActionableMilestone,
// NotActionableDependencies, NotActionableDependents; an empty, non-nil list
// when it is actionable (spec/1.6.0.md F3). A condition is actionable when its
// milestone is open, its direct dependencies are done, and at least one of:
// it is a member (GateMembers) of the gate of an open milestone, so deciding
// it can turn that gate green; nothing depends on it; or an epic depending on
// it can move (not done, in an open milestone or with a bound file) and has
// only conditions among its undone direct dependencies. The first keeps a
// condition that holds a gate from waiting on a dependent in the milestone
// that gate opens. states is DeriveEpicStates' result, gates MilestoneGates'
// and bound EpicBoundFiles'. A condition of a milestone no track lists has a
// closed milestone. tp epic ready, status and the export read this one answer.
func ConditionsNotActionable(g *model.EpicGraph, states map[string]EpicState, gates map[string]MilestoneGate, bound map[string][]string) map[string][]string {
	a := actionability{byID: epicsByID(g), states: states, gates: gates, bound: bound,
		dependents: map[string][]*model.Epic{}, gated: openGateMembers(g, gates)}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			a.dependents[d.ID] = append(a.dependents[d.ID], e)
		}
	}
	out := map[string][]string{}
	for i := range g.Epics {
		c := &g.Epics[i]
		if _, seen := out[c.ID]; seen || !c.Condition || states[c.ID].Status == EpicStatusDone {
			continue
		}
		out[c.ID] = a.reasons(c)
	}
	return out
}

// ConditionsActionable returns, for every condition of g that is not done,
// whether it is actionable: ConditionsNotActionable names no reason.
func ConditionsActionable(g *model.EpicGraph, states map[string]EpicState, gates map[string]MilestoneGate, bound map[string][]string) map[string]bool {
	out := map[string]bool{}
	for id, reasons := range ConditionsNotActionable(g, states, gates, bound) {
		out[id] = len(reasons) == 0
	}
	return out
}

// openGateMembers returns the ids that are members of an open milestone's
// gate.
func openGateMembers(g *model.EpicGraph, gates map[string]MilestoneGate) map[string]bool {
	out := map[string]bool{}
	for m, verdict := range gates {
		if !verdict.Open {
			continue
		}
		members, _ := GateMembers(g, m)
		for _, id := range members {
			out[id] = true
		}
	}
	return out
}

// actionability is what ConditionsNotActionable reads for each condition.
type actionability struct {
	byID       map[string]*model.Epic
	states     map[string]EpicState
	gates      map[string]MilestoneGate
	bound      map[string][]string
	dependents map[string][]*model.Epic
	gated      map[string]bool
}

// reasons lists why the undone condition c is not actionable, every reason
// that holds, in their fixed order.
func (a *actionability) reasons(c *model.Epic) []string {
	out := []string{}
	if !a.gates[c.Milestone].Open {
		out = append(out, NotActionableMilestone)
	}
	if !epicDepsDone(c, a.states) {
		out = append(out, NotActionableDependencies)
	}
	if !a.gated[c.ID] && !a.unblocksADependent(c) {
		out = append(out, NotActionableDependents)
	}
	return out
}

// unblocksADependent reports whether nothing depends on c, or some epic
// depending on it can move and waits on conditions alone.
func (a *actionability) unblocksADependent(c *model.Epic) bool {
	dependents := a.dependents[c.ID]
	if len(dependents) == 0 {
		return true
	}
	for _, w := range dependents {
		if dependentCanMove(w, a.states, a.gates, a.bound) && heldOnlyByConditions(w, a.byID, a.states) {
			return true
		}
	}
	return false
}

// dependentCanMove reports whether deciding a condition could start w: w is
// not done, and its milestone is open or it is worked through a bound file.
func dependentCanMove(w *model.Epic, states map[string]EpicState, gates map[string]MilestoneGate, bound map[string][]string) bool {
	if states[w.ID].Status == EpicStatusDone {
		return false
	}
	return gates[w.Milestone].Open || len(bound[w.ID]) > 0
}

// heldOnlyByConditions reports whether every undone direct dependency of w is
// a condition.
func heldOnlyByConditions(w *model.Epic, byID map[string]*model.Epic, states map[string]EpicState) bool {
	for _, d := range w.DependsOn {
		if states[d.ID].Status == EpicStatusDone {
			continue
		}
		if dep, ok := byID[d.ID]; !ok || !dep.Condition {
			return false
		}
	}
	return true
}

// epicDepsDone reports whether every direct dependency of e is done.
func epicDepsDone(e *model.Epic, states map[string]EpicState) bool {
	for _, dep := range e.DependsOn {
		if states[dep.ID].Status != EpicStatusDone {
			return false
		}
	}
	return true
}

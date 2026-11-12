package engine

import (
	"slices"

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
	// depends on it, and no dependent that can move, other than a deferrable
	// member, waits on conditions alone.
	NotActionableDependents = "dependents"
	// NotActionableDeferrable: its own milestone's gate lists it deferrable
	// (spec/1.8.0.md §7), so no operator list waits on it.
	NotActionableDeferrable = "deferrable"
)

// ConditionsNotActionable returns, for every condition of g that is neither
// done nor moot (MootEpics), the reasons it is not actionable, in the order
// NotActionableMilestone, NotActionableDependencies, NotActionableDependents,
// NotActionableDeferrable; an empty, non-nil list when it is actionable
// (spec/1.6.0.md F3). A condition is actionable when it is no deferrable
// member (IsDeferrableMember), its milestone is open, its direct
// dependencies are done, and at least one of: it is a member (GateMembers) of
// the gate of an open milestone, so deciding it can turn that gate green;
// nothing depends on it; or an epic depending on it that is no deferrable
// member can move (not done, in an open milestone or with a bound file) and
// has only conditions among its undone direct dependencies (spec/1.8.0.md
// §7: deferrable work waits on nothing the operator must decide). An epic
// depends on c through a plain edge or a group naming it, and an unsatisfied
// group counts as a condition when
// groupCountsAsCondition says so (spec/1.7.0.md §7). The first keeps a
// condition that holds a gate from waiting on a dependent in the milestone
// that gate opens. states is DeriveEpicStates' result, gates MilestoneGates'
// and bound EpicBoundFiles'. A condition of a milestone no track lists has a
// closed milestone. tp epic ready, status and the export read this one answer.
func ConditionsNotActionable(g *model.EpicGraph, states map[string]EpicState, gates map[string]MilestoneGate, bound map[string][]string) map[string][]string {
	a := actionability{g: g, byID: epicsByID(g), states: states, gates: gates, bound: bound,
		dependents: actionDependents(g, states), gated: openGateMembers(g, gates)}
	moot := MootEpics(g, states)
	out := map[string][]string{}
	for i := range g.Epics {
		c := &g.Epics[i]
		if _, seen := out[c.ID]; seen || !c.Condition || states[c.ID].Status == EpicStatusDone || moot[c.ID] {
			continue
		}
		out[c.ID] = a.reasons(c)
	}
	return out
}

// actionDependents indexes, for each id, the epics whose depends_on entries
// name it, a plain edge or a group membership. A satisfied group is left
// out: once one member is done its epic leaves the other members'
// actionability (spec/1.7.0.md §7).
func actionDependents(g *model.EpicGraph, states map[string]EpicState) map[string][]*model.Epic {
	out := map[string][]*model.Epic{}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			if d.IsGroup() && epicDepSatisfied(d, states) {
				continue
			}
			for _, id := range d.Names() {
				out[id] = append(out[id], e)
			}
		}
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
	g          *model.EpicGraph
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
	if IsDeferrableMember(a.g, c) {
		out = append(out, NotActionableDeferrable)
	}
	return out
}

// unblocksADependent reports whether nothing depends on c, or some epic
// depending on it that is no deferrable member can move and waits on
// conditions alone.
func (a *actionability) unblocksADependent(c *model.Epic) bool {
	dependents := a.dependents[c.ID]
	if len(dependents) == 0 {
		return true
	}
	for _, w := range dependents {
		if !IsDeferrableMember(a.g, w) && dependentCanMove(w, a.states, a.gates, a.bound) && a.heldOnlyByConditions(w) {
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

// heldOnlyByConditions reports whether every unsatisfied direct dependency
// of w is a condition: a plain edge to one, or a group that counts as one
// (groupCountsAsCondition).
func (a *actionability) heldOnlyByConditions(w *model.Epic) bool {
	for _, d := range w.DependsOn {
		if epicDepSatisfied(d, a.states) {
			continue
		}
		if d.IsGroup() {
			if !a.groupCountsAsCondition(d) {
				return false
			}
			continue
		}
		if dep, ok := a.byID[d.ID]; !ok || !dep.Condition {
			return false
		}
	}
	return true
}

// groupCountsAsCondition reports whether the unsatisfied group d counts as a
// condition (spec/1.7.0.md §7): one of its condition members has its
// milestone open and its own dependencies done, so the operator can satisfy
// it now.
func (a *actionability) groupCountsAsCondition(d model.EpicDep) bool {
	return slices.ContainsFunc(d.Members(), func(id string) bool {
		m, ok := a.byID[id]
		return ok && m.Condition && a.gates[m.Milestone].Open && epicDepsDone(m, a.states)
	})
}

// epicDepsDone reports whether every direct dependency of e is satisfied.
func epicDepsDone(e *model.Epic, states map[string]EpicState) bool {
	for _, dep := range e.DependsOn {
		if !epicDepSatisfied(dep, states) {
			return false
		}
	}
	return true
}

// epicDepSatisfied reports whether the depends_on entry d holds its epic no
// longer: a plain edge once its epic is done, a group once one member is
// (spec/1.7.0.md §7).
func epicDepSatisfied(d model.EpicDep, states map[string]EpicState) bool {
	if d.IsGroup() {
		return slices.ContainsFunc(d.Members(), func(m string) bool { return states[m].Status == EpicStatusDone })
	}
	return states[d.ID].Status == EpicStatusDone
}

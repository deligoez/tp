package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// ConditionsActionable returns, for every condition of g that is not done,
// whether it is actionable (spec/1.6.0.md F3): its direct dependencies are
// done, its milestone is open, and either nothing depends on it or, for at
// least one epic depending on it, every undone direct dependency is a
// condition, so deciding it can unblock work. states is DeriveEpicStates'
// result and gates MilestoneGates'. A condition of a milestone no track lists
// is not actionable. tp epic ready and the export read this one answer.
func ConditionsActionable(g *model.EpicGraph, states map[string]EpicState, gates map[string]MilestoneGate) map[string]bool {
	byID := epicsByID(g)
	dependents := map[string][]*model.Epic{}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			dependents[d.ID] = append(dependents[d.ID], e)
		}
	}
	out := map[string]bool{}
	for i := range g.Epics {
		c := &g.Epics[i]
		if _, seen := out[c.ID]; seen || !c.Condition || states[c.ID].Status == EpicStatusDone {
			continue
		}
		out[c.ID] = conditionActionable(c, dependents[c.ID], byID, states, gates)
	}
	return out
}

// conditionActionable reports whether the undone condition c is actionable
// given the epics depending on it.
func conditionActionable(c *model.Epic, dependents []*model.Epic, byID map[string]*model.Epic, states map[string]EpicState, gates map[string]MilestoneGate) bool {
	if !gates[c.Milestone].Open || !epicDepsDone(c, states) {
		return false
	}
	if len(dependents) == 0 {
		return true
	}
	for _, w := range dependents {
		if heldOnlyByConditions(w, byID, states) {
			return true
		}
	}
	return false
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

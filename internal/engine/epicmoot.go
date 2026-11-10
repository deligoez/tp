package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// MootEpics returns the undone epics of g that are moot (spec/1.7.0.md §7):
// no gate holds it (GateMembers of any milestone), at least one depends_on
// entry names it, every group naming it is satisfied, and every plain edge
// naming it comes from a moot epic, so what only a lost alternative needed
// is moot too. It is the least fixed point of that rule, derived from states
// (DeriveEpicStates' result) on every read: an epic on a dependency cycle no
// lost alternative reaches is not moot. A moot epic is in neither tp epic
// ready nor the operator's conditions.
func MootEpics(g *model.EpicGraph, states map[string]EpicState) map[string]bool {
	held := gateHeldEpics(g)
	namings := map[string]*mootNaming{}
	naming := func(id string) *mootNaming {
		if namings[id] == nil {
			namings[id] = &mootNaming{}
		}
		return namings[id]
	}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			if d.IsGroup() {
				satisfied := epicDepSatisfied(d, states)
				for _, m := range d.Members() {
					naming(m).unsatisfiedGroup = naming(m).unsatisfiedGroup || !satisfied
				}
				continue
			}
			naming(d.ID).plainFrom = append(naming(d.ID).plainFrom, e.ID)
		}
	}
	candidates := []string{}
	for i := range g.Epics {
		id := g.Epics[i].ID
		if n := namings[id]; n != nil && !n.unsatisfiedGroup && !held[id] && states[id].Status != EpicStatusDone {
			candidates = append(candidates, id)
		}
	}
	return mootFixedPoint(candidates, namings)
}

// mootNaming is how the entries of a graph name one epic: whether an
// unsatisfied group does, and the epics whose plain edges do.
type mootNaming struct {
	unsatisfiedGroup bool
	plainFrom        []string
}

// mootFixedPoint grows the moot set from nothing: a candidate joins it once
// every epic with a plain edge to it is in the set, until no candidate
// joins.
func mootFixedPoint(candidates []string, namings map[string]*mootNaming) map[string]bool {
	moot := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for _, id := range candidates {
			if !moot[id] && allMoot(namings[id].plainFrom, moot) {
				moot[id] = true
				grew = true
			}
		}
	}
	return moot
}

// allMoot reports whether every id of ids is in moot.
func allMoot(ids []string, moot map[string]bool) bool {
	for _, id := range ids {
		if !moot[id] {
			return false
		}
	}
	return true
}

// gateHeldEpics returns the ids a gate holds: the members (GateMembers) of
// the gate of every milestone an epic, a track or a gates entry names.
func gateHeldEpics(g *model.EpicGraph) map[string]bool {
	milestones := map[string]bool{}
	for i := range g.Epics {
		milestones[g.Epics[i].Milestone] = true
	}
	for _, track := range g.Milestones {
		for _, m := range track {
			milestones[m] = true
		}
	}
	for m := range g.Gates {
		milestones[m] = true
	}
	out := map[string]bool{}
	for m := range milestones {
		members, _ := GateMembers(g, m)
		for _, id := range members {
			out[id] = true
		}
	}
	return out
}

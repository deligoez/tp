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

package engine

import (
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// DashboardChoice is one any-of group the operator settles by recording one
// of its conditions done: the work epic waiting on it and the group's
// members, in depends_on order (spec/1.8.0.md §7).
type DashboardChoice struct {
	Epic  string   `json:"epic"`
	AnyOf []string `json:"any_of"`
}

// DashboardOperatorGroups lists, in graph order, every unsatisfied any-of
// group whose members are all conditions, held by an undone work epic of an
// open milestone that is not a deferrable member of its gate.
func DashboardOperatorGroups(g *model.EpicGraph, states map[string]EpicState) []DashboardChoice {
	gates := MilestoneGates(g, states)
	byID := epicsByID(g)
	isCondition := func(id string) bool { e, ok := byID[id]; return ok && e.Condition }
	isDone := func(id string) bool { return states[id].Status == EpicStatusDone }
	out := []DashboardChoice{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if !IsWorkEpic(e) || isDone(e.ID) || !gates[e.Milestone].Open || IsDeferrableMember(g, e) {
			continue
		}
		for _, d := range e.DependsOn {
			if !d.IsGroup() {
				continue
			}
			members := d.Members()
			if slices.ContainsFunc(members, isDone) || !allOf(members, isCondition) {
				continue
			}
			out = append(out, DashboardChoice{Epic: e.ID, AnyOf: members})
		}
	}
	return out
}

// allOf reports whether every id satisfies ok.
func allOf(ids []string, ok func(string) bool) bool {
	return !slices.ContainsFunc(ids, func(id string) bool { return !ok(id) })
}

// grantableSlices counts the slices the operator may grant one round.
func grantableSlices(reviewSlices []EpicSliceState) int {
	n := 0
	for i := range reviewSlices {
		if reviewSlices[i].Grantable {
			n++
		}
	}
	return n
}

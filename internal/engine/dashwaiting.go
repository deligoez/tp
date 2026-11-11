package engine

import (
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// IsDeferrableMember reports whether e is listed deferrable in its own
// milestone's gate (spec/1.5.0.md §4): work that waits on a trigger that may
// never come and never holds the gate. The critical path and the dashboard's
// lists read this one predicate.
func IsDeferrableMember(g *model.EpicGraph, e *model.Epic) bool {
	return slices.Contains(g.Gates[e.Milestone].Deferrable, e.ID)
}

// DashboardWaiting returns the dashboard's waiting list for milestone m, or
// for the whole graph when m is empty: the work epics, in graph order, that
// are not done and wait on an undone dependency, less the deferrable members
// (spec/1.8.0.md §7). tp epic status's blocked list and tp epic export keep
// their v1.6/1.7 contract; only the dashboard's list leaves these out.
func DashboardWaiting(g *model.EpicGraph, states map[string]EpicState, m string) []string {
	conditions := make(map[string]bool, len(g.Epics))
	for i := range g.Epics {
		conditions[g.Epics[i].ID] = g.Epics[i].Condition
	}
	out := []string{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if !IsWorkEpic(e) || IsDeferrableMember(g, e) || (m != "" && e.Milestone != m) ||
			states[e.ID].Status == EpicStatusDone {
			continue
		}
		if blocked, _ := epicWaitFlags(e, states, conditions); blocked {
			out = append(out, e.ID)
		}
	}
	return out
}

// DashboardOperatorConditions returns the conditions of actionable, tp epic
// ready's operator conditions, in their order, that the dashboard shows as an
// operator decision (spec/1.8.0.md §7): not a deferrable member itself, and
// either a member of an open milestone's gate, with no undone dependent, or
// waited on by an undone dependent that is no deferrable member. A condition
// whose only undone dependents are deferrable members decides nothing the
// plan waits on.
func DashboardOperatorConditions(g *model.EpicGraph, states map[string]EpicState, actionable []string) []string {
	byID := epicsByID(g)
	dependents := actionDependents(g, states)
	gated := openGateMembers(g, MilestoneGates(g, states))
	out := []string{}
	for _, id := range actionable {
		c, ok := byID[id]
		if !ok || IsDeferrableMember(g, c) {
			continue
		}
		if gated[id] || waitedOnByNonDeferrable(g, states, dependents[id]) {
			out = append(out, id)
		}
	}
	return out
}

// waitedOnByNonDeferrable reports whether dependents holds no undone epic, or
// an undone one that is no deferrable member.
func waitedOnByNonDeferrable(g *model.EpicGraph, states map[string]EpicState, dependents []*model.Epic) bool {
	undone := 0
	for _, w := range dependents {
		if states[w.ID].Status == EpicStatusDone {
			continue
		}
		undone++
		if !IsDeferrableMember(g, w) {
			return true
		}
	}
	return undone == 0
}

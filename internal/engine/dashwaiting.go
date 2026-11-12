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
// (spec/1.8.0.md §7), which tp epic status's and export's blocked lists leave
// out too. Unlike those lists it holds no condition.
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

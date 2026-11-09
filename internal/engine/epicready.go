package engine

import (
	"cmp"
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// EpicReadiness is what can start and what waits on the operator
// (spec/1.5.0.md §7, spec/1.5.0-corrections.md C1). Each list holds ids in
// graph order, Gates milestone names in track order.
type EpicReadiness struct {
	// Ready holds the work epics that can start, most downstream first
	// (spec/1.6.0.md F1): by longest chain, then dependents, then graph order.
	Ready []string
	// Downstream holds each ready epic's downstream.
	Downstream map[string]EpicDownstream
	// Claimed holds the epics that would be ready but for their claim.
	Claimed []string
	// Conditions holds the conditions waiting on the operator: the
	// actionable ones, or with all every condition that is not done.
	Conditions []string
	// Actionable holds ConditionsActionable's answer for every condition
	// that is not done.
	Actionable map[string]bool
	// Gates holds the milestones whose gate members are done and whose
	// threshold has no met record.
	Gates []string
}

// EpicReady sorts g's epics into what can start and what waits on the
// operator. states is DeriveEpicStates' result and bound EpicBoundFiles'.
// A work epic is a candidate when its milestone is in a track and open (any
// tracked milestone with all), it is not done, and every direct dependency is
// done; it is ready when it has no bound file, or claimed when it carries a
// claim. A condition waits on the operator when it is actionable
// (spec/1.6.0.md F3), or with all whenever it is not done. An epic whose
// milestone is in no track is never listed.
func EpicReady(g *model.EpicGraph, states map[string]EpicState, bound map[string][]string, all bool) EpicReadiness {
	gates := MilestoneGates(g, states)
	r := EpicReadiness{Ready: []string{}, Claimed: []string{}, Conditions: []string{}, Gates: []string{}}
	r.Actionable = ConditionsActionable(g, states, gates)
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.Condition {
			if listCondition(e, gates, r.Actionable, all) {
				r.Conditions = append(r.Conditions, e.ID)
			}
			continue
		}
		if !epicCandidate(e, gates, states, all) {
			continue
		}
		switch {
		case len(bound[e.ID]) > 0:
			// A bound epic is worked through its task file.
		case e.Claimed != nil:
			r.Claimed = append(r.Claimed, e.ID)
		default:
			r.Ready = append(r.Ready, e.ID)
		}
	}
	for _, track := range g.Milestones {
		for _, m := range track {
			if gateAwaitsThreshold(g, m, states) {
				r.Gates = append(r.Gates, m)
			}
		}
	}
	r.Downstream = EpicDownstreams(g, states, gates, r.Ready)
	slices.SortStableFunc(r.Ready, func(a, b string) int {
		da, db := r.Downstream[a], r.Downstream[b]
		if c := cmp.Compare(db.LongestChain, da.LongestChain); c != 0 {
			return c
		}
		return cmp.Compare(db.Dependents, da.Dependents)
	})
	return r
}

// listCondition reports whether the condition e is listed: its milestone is
// tracked, it is not done (actionable names it), and it is actionable or all
// is set.
func listCondition(e *model.Epic, gates map[string]MilestoneGate, actionable map[string]bool, all bool) bool {
	if _, tracked := gates[e.Milestone]; !tracked {
		return false
	}
	yes, undone := actionable[e.ID]
	return undone && (all || yes)
}

// epicCandidate reports whether e's milestone is tracked and open (or all),
// e is not done, and every direct dependency of e is done.
func epicCandidate(e *model.Epic, gates map[string]MilestoneGate, states map[string]EpicState, all bool) bool {
	verdict, tracked := gates[e.Milestone]
	if !tracked || (!all && !verdict.Open) || states[e.ID].Status == EpicStatusDone {
		return false
	}
	return epicDepsDone(e, states)
}

// gateAwaitsThreshold reports whether m's gate has a threshold with no met
// record while every member is done: only the operator's measurement holds it.
func gateAwaitsThreshold(g *model.EpicGraph, m string, states map[string]EpicState) bool {
	gate := g.Gates[m]
	return gate.Threshold != "" && gate.Met == nil && gateMembersDone(g, m, states)
}

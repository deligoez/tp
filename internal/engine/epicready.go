package engine

import "github.com/deligoez/tp/internal/model"

// EpicReadiness is what can start and what waits on the operator
// (spec/1.5.0.md §7, spec/1.5.0-corrections.md C1). Each list holds ids in
// graph order, Gates milestone names in track order.
type EpicReadiness struct {
	// Ready holds the work epics that can start.
	Ready []string
	// Claimed holds the epics that would be ready but for their claim.
	Claimed []string
	// Conditions holds the conditions waiting on the operator.
	Conditions []string
	// Gates holds the milestones whose gate members are done and whose
	// threshold has no met record.
	Gates []string
}

// EpicReady sorts g's epics into what can start and what waits on the
// operator. states is DeriveEpicStates' result and bound EpicBoundFiles'.
// An epic is a candidate when its milestone is in a track and open (any
// tracked milestone with all), it is not done, and every direct dependency is
// done. A candidate work epic with no bound file is ready, or claimed when it
// carries a claim; a candidate condition waits on the operator. An epic whose
// milestone is in no track is never listed.
func EpicReady(g *model.EpicGraph, states map[string]EpicState, bound map[string][]string, all bool) EpicReadiness {
	gates := MilestoneGates(g, states)
	r := EpicReadiness{Ready: []string{}, Claimed: []string{}, Conditions: []string{}, Gates: []string{}}
	for i := range g.Epics {
		e := &g.Epics[i]
		if !epicCandidate(e, gates, states, all) {
			continue
		}
		switch {
		case e.Condition:
			r.Conditions = append(r.Conditions, e.ID)
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
	return r
}

// epicCandidate reports whether e's milestone is tracked and open (or all),
// e is not done, and every direct dependency of e is done.
func epicCandidate(e *model.Epic, gates map[string]MilestoneGate, states map[string]EpicState, all bool) bool {
	verdict, tracked := gates[e.Milestone]
	if !tracked || (!all && !verdict.Open) || states[e.ID].Status == EpicStatusDone {
		return false
	}
	for _, dep := range e.DependsOn {
		if states[dep.ID].Status != EpicStatusDone {
			return false
		}
	}
	return true
}

// gateAwaitsThreshold reports whether m's gate has a threshold with no met
// record while every member is done: only the operator's measurement holds it.
func gateAwaitsThreshold(g *model.EpicGraph, m string, states map[string]EpicState) bool {
	gate := g.Gates[m]
	return gate.Threshold != "" && gate.Met == nil && gateMembersDone(g, m, states)
}

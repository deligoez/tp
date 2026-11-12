package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// EpicScope is what one milestone's gate pulls in (spec/backlog/epic-scope.md
// §1, Part A), derived from the graph and writing nothing.
type EpicScope struct {
	Milestone string `json:"milestone"`
	// Members is the gate's effective members (GateMembers); MembersImplicit
	// says the gate names none, so they are every epic of the milestone that
	// is neither a condition nor deferrable.
	Members         []string `json:"members"`
	MembersImplicit bool     `json:"members_implicit"`
}

// MilestoneScope derives milestone's scope over states, DeriveEpicStates'
// result, and estimates, EpicEstimates' (nil leaves the critical path by
// estimate out).
func MilestoneScope(g *model.EpicGraph, states map[string]EpicState, estimates map[string]int, milestone string) EpicScope {
	members, _ := GateMembers(g, milestone)
	return EpicScope{
		Milestone:       milestone,
		Members:         members,
		MembersImplicit: len(g.Gates[milestone].Members) == 0,
	}
}

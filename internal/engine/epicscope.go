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
	// Closure is the not-done epics the not-done members reach, members
	// included, per milestone in milestone order (milestoneOrder), listing
	// only milestones with at least one.
	Closure []ScopeMilestone `json:"closure"`
}

// ScopeMilestone is one milestone's part of a closure: its track's index in
// the graph's tracks (-1 for a milestone no track lists), and its epics in
// graph order.
type ScopeMilestone struct {
	Milestone string      `json:"milestone"`
	Track     int         `json:"track"`
	Size      int         `json:"size"`
	Epics     []ScopeEpic `json:"epics"`
}

// ScopeEpic is one epic of a closure.
type ScopeEpic struct {
	ID string `json:"id"`
}

// MilestoneScope derives milestone's scope over states, DeriveEpicStates'
// result, and estimates, EpicEstimates' (nil leaves the critical path by
// estimate out).
func MilestoneScope(g *model.EpicGraph, states map[string]EpicState, estimates map[string]int, milestone string) EpicScope {
	w := newScopeWalk(g, states)
	members, _ := GateMembers(g, milestone)
	full := w.closure(members, "")
	return EpicScope{
		Milestone:       milestone,
		Members:         members,
		MembersImplicit: len(g.Gates[milestone].Members) == 0,
		Closure:         w.compose(full),
	}
}

// scopeWalk walks depends_on edges backwards from a gate's members through
// not-done epics.
type scopeWalk struct {
	g      *model.EpicGraph
	states map[string]EpicState
	epics  map[string]*model.Epic
	pos    map[string]milestonePos
}

func newScopeWalk(g *model.EpicGraph, states map[string]EpicState) *scopeWalk {
	return &scopeWalk{g: g, states: states, epics: epicsByID(g), pos: milestonePositions(g)}
}

// scopeSet is one closure: the epics in it.
type scopeSet struct {
	in map[string]bool
}

// open reports whether id names an epic of the graph that is not done.
func (w *scopeWalk) open(id string) bool {
	_, ok := w.epics[id]
	return ok && w.states[id].Status != EpicStatusDone
}

// closure returns the not-done epics members reach, the not-done members
// included, over depends_on entries that still hold, never entering cut: an
// empty cut cuts nothing.
func (w *scopeWalk) closure(members []string, cut string) scopeSet {
	set := scopeSet{in: map[string]bool{}}
	queue := []string{}
	add := func(id string) {
		if (cut != "" && id == cut) || !w.open(id) || set.in[id] {
			return
		}
		set.in[id] = true
		queue = append(queue, id)
	}
	for _, id := range members {
		add(id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, d := range w.epics[id].DependsOn {
			if !epicDepSatisfied(d, w.states) {
				add(d.ID)
			}
		}
	}
	return set
}

// compose lays set out per milestone in milestone order, each milestone's
// epics in graph order.
func (w *scopeWalk) compose(set scopeSet) []ScopeMilestone {
	byMilestone := map[string][]ScopeEpic{}
	for i := range w.g.Epics {
		e := &w.g.Epics[i]
		if set.in[e.ID] && w.epics[e.ID] == e {
			byMilestone[e.Milestone] = append(byMilestone[e.Milestone], ScopeEpic{ID: e.ID})
		}
	}
	out := []ScopeMilestone{}
	for _, m := range milestoneOrder(w.g) {
		epics := byMilestone[m]
		if len(epics) == 0 {
			continue
		}
		out = append(out, ScopeMilestone{Milestone: m, Track: w.track(m), Size: len(epics), Epics: epics})
	}
	return out
}

// track returns m's track index, -1 for a milestone no track lists.
func (w *scopeWalk) track(m string) int {
	if p, ok := w.pos[m]; ok {
		return p.track
	}
	return -1
}

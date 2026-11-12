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

// ScopeEpic is one epic of a closure. AnyOf is the unsatisfied any-of group
// that brought it in, the first the walk met, when only groups did: an epic
// a member is, or a plain edge reaches, is needed whatever the operator
// chooses and carries none.
type ScopeEpic struct {
	ID    string   `json:"id"`
	AnyOf []string `json:"any_of,omitempty"`
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

// scopeSet is one closure: the epics in it, and the any-of group each epic
// only groups brought in came by (ScopeEpic.AnyOf).
type scopeSet struct {
	in    map[string]bool
	anyOf map[string][]string
}

// open reports whether id names an epic of the graph that is not done.
func (w *scopeWalk) open(id string) bool {
	_, ok := w.epics[id]
	return ok && w.states[id].Status != EpicStatusDone
}

// closure returns the not-done epics members reach, the not-done members
// included, over depends_on entries that still hold, never entering cut: an
// empty cut cuts nothing. A plain edge leads to its epic, an unsatisfied
// any-of group to every member, a satisfied one nowhere (spec/1.7.0.md §7).
// A moot epic needs no check of its own, as on CriticalPath's chains: a gate
// holds none, only a moot epic has a plain edge to one, and a satisfied group
// is not walked.
func (w *scopeWalk) closure(members []string, cut string) scopeSet {
	set := scopeSet{in: map[string]bool{}, anyOf: map[string][]string{}}
	plain := map[string]bool{}
	queue := []string{}
	add := func(id string, group []string) {
		if (cut != "" && id == cut) || !w.open(id) {
			return
		}
		if group == nil {
			plain[id] = true
		} else if set.anyOf[id] == nil {
			set.anyOf[id] = group
		}
		if !set.in[id] {
			set.in[id] = true
			queue = append(queue, id)
		}
	}
	for _, id := range members {
		add(id, nil)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, d := range w.epics[id].DependsOn {
			w.follow(d, add)
		}
	}
	for id := range plain {
		delete(set.anyOf, id)
	}
	return set
}

// follow hands add the epics one depends_on entry still holds its epic on:
// a plain edge's epic, or every member of an unsatisfied group with the
// group.
func (w *scopeWalk) follow(d model.EpicDep, add func(id string, group []string)) {
	if epicDepSatisfied(d, w.states) {
		return
	}
	if !d.IsGroup() {
		add(d.ID, nil)
		return
	}
	group := d.Members()
	for _, id := range group {
		add(id, group)
	}
}

// compose lays set out per milestone in milestone order, each milestone's
// epics in graph order.
func (w *scopeWalk) compose(set scopeSet) []ScopeMilestone {
	byMilestone := map[string][]ScopeEpic{}
	for i := range w.g.Epics {
		e := &w.g.Epics[i]
		if set.in[e.ID] && w.epics[e.ID] == e {
			byMilestone[e.Milestone] = append(byMilestone[e.Milestone], ScopeEpic{ID: e.ID, AnyOf: set.anyOf[e.ID]})
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

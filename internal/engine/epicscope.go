package engine

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicScope is what one milestone's gate pulls in (spec/backlog/epic-scope.md
// §1, Part A), derived from the graph and writing nothing.
type EpicScope struct {
	Milestone string `json:"milestone"`
	// Fingerprint hashes the member set, the id set of the closure drawn
	// blind to status (blindClosure) and the threshold text, so "unchanged
	// since the last scope pass" is a string compare that closing an epic
	// does not move, and a rewritten threshold is a change even when the
	// closure is identical.
	Fingerprint string `json:"fingerprint"`
	// Members is the gate's effective members (GateMembers); MembersImplicit
	// says the gate names none, so they are every epic of the milestone that
	// is neither a condition nor deferrable.
	Members         []string `json:"members"`
	MembersImplicit bool     `json:"members_implicit"`
	// Closure is the not-done epics the not-done members reach, members
	// included, per milestone in milestone order (milestoneOrder), listing
	// only milestones with at least one.
	Closure []ScopeMilestone `json:"closure"`
	// Shrink is each closure epic's removal shrink that takes at least one
	// other epic with it, by size descending, then id.
	Shrink []ScopeShrink `json:"shrink"`
	// Outside is the milestone's not-done epics the closure misses, in graph
	// order. Under implicit members it is empty by definition and
	// OutsideImplicit says so, rather than leaving [] to read as a measured
	// result.
	Outside         []string `json:"outside"`
	OutsideImplicit bool     `json:"outside_implicit,omitempty"`
	// NoDependents is the part of Outside no epic of the graph depends on, by
	// plain edge or as an any-of member; empty under implicit members.
	NoDependents []string `json:"no_dependents"`
	// UnneededEarlier is, for each milestone before this one in its track,
	// in track order, the not-done members of its gate that no closure of a
	// later gate of the track contains; a milestone with none is left out.
	UnneededEarlier []ScopeUnneeded `json:"unneeded_earlier"`
	// CriticalPath is the milestone's own critical path (CriticalPath), the
	// command's to render as tp epic status renders it.
	CriticalPath EpicCriticalPath `json:"-"`
}

// ScopeUnneeded is one earlier milestone's gate members no later gate of the
// track needs, in gate order. ImplicitLater names the later milestones whose
// gates were implicit: such a closure holds its whole milestone, so it says
// nothing about which of that milestone's epics are needed.
type ScopeUnneeded struct {
	Milestone     string   `json:"milestone"`
	Epics         []string `json:"epics"`
	ImplicitLater []string `json:"implicit_later"`
}

// ScopeShrink is one closure epic's removal shrink: for a non-member, the
// epics that leave the closure when every edge into it is cut, itself
// included; for a member, the epics that leave when it is dropped from the
// gate, a gate decision. Leaves is sorted by id and Size is its length.
type ScopeShrink struct {
	Epic   string   `json:"epic"`
	Member bool     `json:"member"`
	Size   int      `json:"size"`
	Leaves []string `json:"leaves"`
}

// ScopeMilestone is one milestone's part of a closure: its track's index in
// the graph's tracks (-1 for a milestone no track lists), and its epics in
// graph order. CrossTrack says the track is not the scope milestone's:
// information, which lint leaves free by design, never an error.
type ScopeMilestone struct {
	Milestone  string      `json:"milestone"`
	Track      int         `json:"track"`
	CrossTrack bool        `json:"cross_track,omitempty"`
	Size       int         `json:"size"`
	Epics      []ScopeEpic `json:"epics"`
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
	w := newScopeWalk(g, states, milestone)
	members, _ := GateMembers(g, milestone)
	full := w.closure(members, "")
	closure := w.compose(full)
	implicit := len(g.Gates[milestone].Members) == 0
	s := EpicScope{
		Milestone:       milestone,
		Members:         members,
		MembersImplicit: implicit,
		Closure:         closure,
		Shrink:          w.shrink(members, full, closure),
		Outside:         []string{},
		OutsideImplicit: implicit,
		NoDependents:    []string{},
		UnneededEarlier: w.unneededEarlier(milestone),
		CriticalPath:    CriticalPath(g, states, estimates, milestone),
	}
	if !implicit {
		s.Outside, s.NoDependents = w.outside(milestone, full)
	}
	s.Fingerprint = scopeFingerprint(members, blindClosure(w.epics, members), g.Gates[milestone].Threshold)
	return s
}

// scopeFingerprint hashes members in order, the ids of full sorted, path in
// order and threshold, each id quoted (strconv.Quote) and each list on a line
// of its own, so no two inputs share one canonical string.
// scopeFingerprint hashes the members sorted, the status-blind closure's ids
// sorted, and the threshold. Each part is labelled and counted and each item
// length-prefixed, so an id moving between parts cannot collide and a new
// part is an additive change.
func scopeFingerprint(members, closure []string, threshold string) string {
	var b strings.Builder
	part := func(label string, items []string) {
		fmt.Fprintf(&b, "%s:%d\n", label, len(items))
		for _, s := range items {
			fmt.Fprintf(&b, "%d:%s\n", len(s), s)
		}
	}
	part("members", slices.Sorted(slices.Values(members)))
	part("closure", closure)
	part("threshold", []string{threshold})
	return epicHash(b.String())
}

// blindClosure returns, sorted, the ids members reach with every epic taken
// as not done and every any-of member followed, satisfied groups included:
// the closure as the graph's shape alone draws it, so closing an epic leaves
// the fingerprint where it was.
func blindClosure(epics map[string]*model.Epic, members []string) []string {
	in := map[string]bool{}
	queue := []string{}
	add := func(id string) {
		if _, ok := epics[id]; ok && !in[id] {
			in[id] = true
			queue = append(queue, id)
		}
	}
	for _, id := range members {
		add(id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, d := range epics[id].DependsOn {
			add(d.ID)
			for _, m := range d.Members() {
				add(m)
			}
		}
	}
	return slices.Sorted(maps.Keys(in))
}

// scopeWalk walks depends_on edges backwards from a gate's members through
// not-done epics.
type scopeWalk struct {
	g      *model.EpicGraph
	states map[string]EpicState
	epics  map[string]*model.Epic
	pos    map[string]milestonePos
	// home is the scope milestone's track.
	home int
}

func newScopeWalk(g *model.EpicGraph, states map[string]EpicState, milestone string) *scopeWalk {
	w := &scopeWalk{g: g, states: states, epics: epicsByID(g), pos: milestonePositions(g)}
	w.home = w.track(milestone)
	return w
}

// scopeSet is one closure: the epics in it, and the any-of group each epic
// only groups brought in came by (ScopeEpic.AnyOf).
type scopeSet struct {
	in    map[string]bool
	anyOf map[string][]string
}

// unneededEarlier returns the ScopeUnneeded rows of the milestones before
// milestone in its track; none for a milestone no track lists.
func (w *scopeWalk) unneededEarlier(milestone string) []ScopeUnneeded {
	out := []ScopeUnneeded{}
	p, ok := w.pos[milestone]
	if !ok {
		return out
	}
	track := w.g.Milestones[p.track]
	closures := map[string]scopeSet{}
	for _, m := range track {
		if _, done := closures[m]; !done {
			members, _ := GateMembers(w.g, m)
			closures[m] = w.closure(members, "")
		}
	}
	for i, e := range track[:p.index] {
		row := ScopeUnneeded{Milestone: e, Epics: []string{}, ImplicitLater: []string{}}
		later := track[i+1:]
		for _, l := range later {
			if len(w.g.Gates[l].Members) == 0 && !slices.Contains(row.ImplicitLater, l) {
				row.ImplicitLater = append(row.ImplicitLater, l)
			}
		}
		members, _ := GateMembers(w.g, e)
		for _, id := range members {
			if w.open(id) && !slices.ContainsFunc(later, func(l string) bool { return closures[l].in[id] }) {
				row.Epics = append(row.Epics, id)
			}
		}
		if len(row.Epics) > 0 {
			out = append(out, row)
		}
	}
	return out
}

// outside returns milestone's not-done epics full misses, in graph order,
// and the part of them no depends_on entry of the graph names.
func (w *scopeWalk) outside(milestone string, full scopeSet) (outside, noDependents []string) {
	named := map[string]bool{}
	for i := range w.g.Epics {
		for _, d := range w.g.Epics[i].DependsOn {
			named[d.ID] = true
			for _, id := range d.Members() {
				named[id] = true
			}
		}
	}
	outside, noDependents = []string{}, []string{}
	for i := range w.g.Epics {
		e := &w.g.Epics[i]
		if e.Milestone != milestone || w.epics[e.ID] != e || !w.open(e.ID) || full.in[e.ID] {
			continue
		}
		outside = append(outside, e.ID)
		if !named[e.ID] {
			noDependents = append(noDependents, e.ID)
		}
	}
	return outside, noDependents
}

// shrink returns the removal shrink rows of full's epics, listed in closure
// order, that take another epic with them.
func (w *scopeWalk) shrink(members []string, full scopeSet, closure []ScopeMilestone) []ScopeShrink {
	out := []ScopeShrink{}
	for _, m := range closure {
		for _, e := range m.Epics {
			row := ScopeShrink{Epic: e.ID, Member: slices.Contains(members, e.ID)}
			var rest scopeSet
			if row.Member {
				rest = w.closure(slices.DeleteFunc(slices.Clone(members), func(id string) bool { return id == e.ID }), "")
			} else {
				rest = w.closure(members, e.ID)
			}
			row.Leaves = full.minus(rest)
			row.Size = len(row.Leaves)
			if slices.ContainsFunc(row.Leaves, func(id string) bool { return id != e.ID }) {
				out = append(out, row)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Epic < out[j].Epic
	})
	return out
}

// minus returns the ids of s not in o, sorted.
func (s scopeSet) minus(o scopeSet) []string {
	out := []string{}
	for id := range s.in {
		if !o.in[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
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
		track := w.track(m)
		out = append(out, ScopeMilestone{Milestone: m, Track: track, CrossTrack: track != w.home, Size: len(epics), Epics: epics})
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

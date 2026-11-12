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
	// Outside is the milestone's not-done epics the closure misses, moot ones
	// left out, in graph order, computed the same way under implicit members,
	// which leave the deferrable and condition epics out.
	Outside []ScopeOutside `json:"outside"`
	// NoDependents is the ids of Outside no epic of the graph depends on, by
	// plain edge or as an any-of member.
	NoDependents []string `json:"no_dependents"`
	// DeferrableInClosure is the gate's deferrable epics the closure
	// requires (ScopeEpic): the members reach them through plain edges
	// alone, so they cannot actually be deferred. In the gate's deferrable
	// order, [] when none.
	DeferrableInClosure []string `json:"deferrable_in_closure"`
	// UnneededEarlier is, for each milestone before this one in its track,
	// in track order, the not-done members of its gate that the closure of
	// no other gate of the graph, any track, contains; a milestone with none
	// is left out.
	UnneededEarlier []ScopeUnneeded `json:"unneeded_earlier"`
	// CriticalPath is the milestone's own critical path (CriticalPath), the
	// command's to render as tp epic status renders it.
	CriticalPath EpicCriticalPath `json:"-"`
}

// ScopeOutside is one epic outside the closure, marked when its gate lists
// it deferrable or it is a condition.
type ScopeOutside struct {
	ID         string `json:"id"`
	Deferrable bool   `json:"deferrable,omitempty"`
	Condition  bool   `json:"condition,omitempty"`
}

// ScopeUnneeded is one earlier milestone's gate members no other gate of the
// graph needs, in gate order. ImplicitGates names the other gates consulted
// that were implicit, in track order: such a closure holds its whole
// milestone, so it says nothing about which of that milestone's epics are
// needed.
type ScopeUnneeded struct {
	Milestone     string   `json:"milestone"`
	Epics         []string `json:"epics"`
	ImplicitGates []string `json:"implicit_gates"`
}

// ScopeShrink is one closure epic's removal shrink: for a non-member, the
// epics that leave the closure when every edge into it is cut, itself
// included; for a member, the epics that leave when it is dropped from the
// gate, a gate decision. AnyOf is the epic's any-of mark in the closure
// (ScopeEpic.AnyOf): cutting it is choosing another alternative. Leaves is
// sorted by id and Size is its length, the epic itself counted.
type ScopeShrink struct {
	Epic   string   `json:"epic"`
	Member bool     `json:"member"`
	AnyOf  []string `json:"any_of,omitempty"`
	Size   int      `json:"size"`
	Leaves []string `json:"leaves"`
}

// ScopeMilestone is one milestone's part of a closure: its track's index in
// the graph's tracks (absent for a milestone no track lists), and its epics
// in graph order. CrossTrack says the milestone, not the scope's own, shares
// no track with it, as two untracked milestones never do: information, which
// lint leaves free by design, never an error.
type ScopeMilestone struct {
	Milestone  string      `json:"milestone"`
	Track      *int        `json:"track,omitempty"`
	CrossTrack bool        `json:"cross_track,omitempty"`
	Size       int         `json:"size"`
	Epics      []ScopeEpic `json:"epics"`
}

// ScopeEpic is one epic of a closure. A required epic, one the not-done
// members reach through unsatisfied plain edges alone, carries no AnyOf: it
// is needed whatever the operator chooses. Every other epic is optional and
// AnyOf is the group of the alternative it hangs from: the group that
// reached it, or the mark of the epic whose plain edge did, the first in
// walk order when two groups reach it.
type ScopeEpic struct {
	ID    string   `json:"id"`
	AnyOf []string `json:"any_of,omitempty"`
}

// MilestoneScope derives milestone's scope over states, DeriveEpicStates'
// result, and estimates, EpicEstimates' (nil leaves the critical path by
// estimate out).
func MilestoneScope(g *model.EpicGraph, states map[string]EpicState, estimates map[string]int, milestone string) EpicScope {
	w := newScopeWalk(g, states, milestone)
	members, deferrable := GateMembers(g, milestone)
	full := w.closure(members, "")
	closure := w.compose(full)
	s := EpicScope{
		Milestone:           milestone,
		Members:             members,
		MembersImplicit:     len(g.Gates[milestone].Members) == 0,
		Closure:             closure,
		Shrink:              w.shrink(members, full, closure),
		DeferrableInClosure: full.required(deferrable),
		UnneededEarlier:     w.unneededEarlier(milestone),
		CriticalPath:        CriticalPath(g, states, estimates, milestone),
	}
	s.Outside, s.NoDependents = w.outside(milestone, deferrable, full)
	s.Fingerprint = scopeFingerprint(members, blindClosure(w.epics, members), g.Gates[milestone].Threshold)
	return s
}

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
	// moot marks each moot epic (MootEpics), which counts nowhere.
	moot map[string]bool
	// milestone is the scope's milestone, home its track (nil untracked).
	milestone string
	home      *int
}

func newScopeWalk(g *model.EpicGraph, states map[string]EpicState, milestone string) *scopeWalk {
	w := &scopeWalk{g: g, states: states, epics: epicsByID(g), pos: milestonePositions(g), moot: MootEpics(g, states), milestone: milestone}
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
// milestone in its track, each member checked against the closure of every
// other tracked gate, any track; none for a milestone no track lists.
func (w *scopeWalk) unneededEarlier(milestone string) []ScopeUnneeded {
	out := []ScopeUnneeded{}
	p, ok := w.pos[milestone]
	if !ok {
		return out
	}
	gates := w.trackedGates()
	for _, e := range w.g.Milestones[p.track][:p.index] {
		row := ScopeUnneeded{Milestone: e, Epics: []string{}, ImplicitGates: []string{}}
		for _, o := range gates {
			if o.milestone != e && o.implicit {
				row.ImplicitGates = append(row.ImplicitGates, o.milestone)
			}
		}
		members, _ := GateMembers(w.g, e)
		for _, id := range members {
			if w.open(id) && !slices.ContainsFunc(gates, func(o scopeGate) bool { return o.milestone != e && o.closure.in[id] }) {
				row.Epics = append(row.Epics, id)
			}
		}
		if len(row.Epics) > 0 {
			out = append(out, row)
		}
	}
	return out
}

// scopeGate is one tracked milestone's gate: whether its members are
// implicit, and its closure.
type scopeGate struct {
	milestone string
	implicit  bool
	closure   scopeSet
}

// trackedGates returns the gate of every milestone a track lists, once, in
// track order.
func (w *scopeWalk) trackedGates() []scopeGate {
	out := []scopeGate{}
	for _, track := range w.g.Milestones {
		for _, m := range track {
			if slices.ContainsFunc(out, func(o scopeGate) bool { return o.milestone == m }) {
				continue
			}
			members, _ := GateMembers(w.g, m)
			out = append(out, scopeGate{milestone: m, implicit: len(w.g.Gates[m].Members) == 0, closure: w.closure(members, "")})
		}
	}
	return out
}

// outside returns milestone's not-done epics full misses, in graph order,
// each marked deferrable or condition, and the ids of those no depends_on
// entry of the graph names.
func (w *scopeWalk) outside(milestone string, deferrable []string, full scopeSet) (outside []ScopeOutside, noDependents []string) {
	named := map[string]bool{}
	for i := range w.g.Epics {
		for _, d := range w.g.Epics[i].DependsOn {
			named[d.ID] = true
			for _, id := range d.Members() {
				named[id] = true
			}
		}
	}
	outside, noDependents = []ScopeOutside{}, []string{}
	for i := range w.g.Epics {
		e := &w.g.Epics[i]
		if e.Milestone != milestone || w.epics[e.ID] != e || !w.open(e.ID) || full.in[e.ID] {
			continue
		}
		outside = append(outside, ScopeOutside{ID: e.ID, Deferrable: slices.Contains(deferrable, e.ID), Condition: e.Condition})
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
			row := ScopeShrink{Epic: e.ID, Member: slices.Contains(members, e.ID), AnyOf: e.AnyOf}
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

// required returns the ids of ids s requires, in their order: in s and with
// no any-of mark (ScopeEpic.AnyOf). Never nil.
func (s scopeSet) required(ids []string) []string {
	out := []string{}
	for _, id := range ids {
		if s.in[id] && s.anyOf[id] == nil {
			out = append(out, id)
		}
	}
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

// open reports whether id names an epic of the graph that is not done and
// not moot: a moot epic leaves every list as a done one does (spec/1.7.0.md
// §7).
func (w *scopeWalk) open(id string) bool {
	_, ok := w.epics[id]
	return ok && w.states[id].Status != EpicStatusDone && !w.moot[id]
}

// closure returns the not-done epics members reach, the not-done members
// included, over depends_on entries that still hold, never entering cut: an
// empty cut cuts nothing. A plain edge leads to its epic, an unsatisfied
// any-of group to every member, a satisfied one nowhere (spec/1.7.0.md §7).
// A moot epic is never entered (open). The required part, what plain edges
// alone reach, carries no mark; every other epic is optional and keeps the
// mark walk gave it.
func (w *scopeWalk) closure(members []string, cut string) scopeSet {
	set := w.walk(members, cut, false)
	for id := range w.walk(members, cut, true).in {
		delete(set.anyOf, id)
	}
	return set
}

// walk is closure's breadth-first pass: plainOnly follows plain edges alone.
// An epic takes its mark when first reached, in walk order: the group of an
// any-of entry that reaches it, or through a plain edge the mark of the epic
// it hangs from, so what only an alternative needs carries that
// alternative's group.
func (w *scopeWalk) walk(members []string, cut string, plainOnly bool) scopeSet {
	set := scopeSet{in: map[string]bool{}, anyOf: map[string][]string{}}
	queue := []string{}
	add := func(id string, mark []string) {
		if (cut != "" && id == cut) || !w.open(id) || set.in[id] {
			return
		}
		set.in[id] = true
		if mark != nil {
			set.anyOf[id] = mark
		}
		queue = append(queue, id)
	}
	for _, id := range members {
		add(id, nil)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, d := range w.epics[id].DependsOn {
			if !epicDepSatisfied(d, w.states) && (!plainOnly || !d.IsGroup()) {
				w.follow(d, set.anyOf[id], add)
			}
		}
	}
	return set
}

// follow hands add the epics one unsatisfied depends_on entry names: a plain
// edge's epic with the mark it inherits, or every member of a group with the
// group.
func (w *scopeWalk) follow(d model.EpicDep, inherited []string, add func(id string, mark []string)) {
	if !d.IsGroup() {
		add(d.ID, inherited)
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
		cross := m != w.milestone && (track == nil || w.home == nil || *track != *w.home)
		out = append(out, ScopeMilestone{Milestone: m, Track: track, CrossTrack: cross, Size: len(epics), Epics: epics})
	}
	return out
}

// track returns m's track index, nil for a milestone no track lists.
func (w *scopeWalk) track(m string) *int {
	if p, ok := w.pos[m]; ok {
		return &p.track
	}
	return nil
}

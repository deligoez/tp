package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// depEdges maps each epic id, by its first definition, to the ids its
// depends_on edges name that the graph holds, leaving out the edges keep
// refuses.
func depEdges(g *model.EpicGraph, keep func(e *model.Epic, target string) bool) map[string][]string {
	byID := epicsByID(g)
	adj := map[string][]string{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if byID[e.ID] != e {
			continue
		}
		for _, d := range e.DependsOn {
			if _, ok := byID[d.ID]; ok && keep(e, d.ID) {
				adj[e.ID] = append(adj[e.ID], d.ID)
			}
		}
	}
	return adj
}

// reachFrom walks adj breadth-first from src and returns the ids it reaches
// in that order, src excluded unless a cycle leads back to it, and each
// reached id's predecessor on a shortest path. follow tells whether the walk
// goes on past a reached id.
func reachFrom(src string, adj map[string][]string, follow func(id string) bool) (order []string, parent map[string]string) {
	parent = map[string]string{}
	queue := []string{src}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if _, seen := parent[next]; seen {
				continue
			}
			parent[next] = cur
			order = append(order, next)
			if next != src && follow(next) {
				queue = append(queue, next)
			}
		}
	}
	return order, parent
}

// reachPath is the shortest path reachFrom found from src to dst, as
// "src -> ... -> dst".
func reachPath(parent map[string]string, src, dst string) string {
	path := []string{dst}
	for cur := dst; ; {
		cur = parent[cur]
		path = append(path, cur)
		if cur == src {
			break
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return strings.Join(path, " -> ")
}

// waitEdge is one edge of the milestone wait graph from a gate to an
// opening: members of milestone gate's gate reaching target, an undone epic
// of milestone opens, through epics without a done record; path is the
// first member's.
type waitEdge struct {
	gate, opens, target string
	members             []string
	path                string
}

// opensLaterHit is one dependency-opens-later warning being built: epics of
// milestone gate outside its members reaching target, of the milestone later
// that opens after gate. from lists the reaching epics in the order found,
// path the first one's.
type opensLaterHit struct {
	gate, later, target string
	from                []string
	path                string
}

// opensLaterScan holds what lintEpicDependencyOpensLater reads and what it
// has found so far: the wait graph's gate-to-opening edges by gate, in the
// order found, and the warnings keyed by epic and target.
type opensLaterScan struct {
	g          *model.EpicGraph
	pos        map[string]milestonePos
	byID       map[string]string
	after      map[string][]string
	milestones []string
	anc        map[string]map[string]bool
	members    map[string]map[string]bool
	edges      []*waitEdge
	from       map[string][]*waitEdge
	warns      []*opensLaterHit
	at         map[string]*opensLaterHit
}

// lintEpicDependencyOpensLater reports each cycle of the milestone wait
// graph over the tracked milestones. A milestone's opening waits on the gate
// of each milestone it opens after, transitively over the effective
// opens_after; a milestone's gate waits on the opening of each other
// milestone B when an effective member of the gate reaches an undone epic of
// B through depends_on and epics without a done record, a condition like any
// epic. A cycle is an error: no gate on it can go green. One finding per
// cycle, in canonical rotation. When an epic of such a milestone outside its
// gate's members reaches an epic of a milestone opening after it and no
// error names that epic, it is a warning: it can still finish once that
// milestone opens. An edge later-milestone-dependency reports, to a later
// milestone of the same track, is that rule's and is not followed.
func lintEpicDependencyOpensLater(g *model.EpicGraph) []EpicLintFinding {
	s := &opensLaterScan{
		g: g, pos: milestonePositions(g), byID: epicMilestones(g), after: EffectiveOpensAfter(g),
		anc: map[string]map[string]bool{}, members: map[string]map[string]bool{},
		from: map[string][]*waitEdge{}, at: map[string]*opensLaterHit{},
	}
	s.milestones = trackedMilestones(g)
	epics := epicsByID(g)
	undone := func(id string) bool { return epics[id].Done == nil }
	adj := depEdges(g, s.keepEdge)
	for i := range g.Epics {
		e := &g.Epics[i]
		if _, tracked := s.pos[e.Milestone]; !tracked || epics[e.ID] != e || !undone(e.ID) {
			continue
		}
		order, parent := reachFrom(e.ID, adj, undone)
		for _, b := range order {
			if undone(b) {
				s.record(e, b, parent)
			}
		}
	}
	return s.findings()
}

// trackedMilestones lists the milestones the tracks name, in track order,
// once each.
func trackedMilestones(g *model.EpicGraph) []string {
	var out []string
	seen := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// keepEdge leaves out the edges later-milestone-dependency reports.
func (s *opensLaterScan) keepEdge(e *model.Epic, target string) bool {
	from, ok := s.pos[e.Milestone]
	to, placed := s.pos[s.byID[target]]
	return !ok || !placed || to.track != from.track || to.index <= from.index
}

// opensAfter reports whether milestone later opens after milestone m,
// transitively.
func (s *opensLaterScan) opensAfter(later, m string) bool {
	if s.anc[later] == nil {
		s.anc[later] = opensAfterClosure(s.after, later)
	}
	return later != m && s.anc[later][m]
}

// record notes src reaching the undone epic b of another tracked milestone:
// a wait edge when src is a member of its milestone's gate, else a warning
// when b's milestone opens after src's.
func (s *opensLaterScan) record(src *model.Epic, b string, parent map[string]string) {
	gate, opens := src.Milestone, s.byID[b]
	if _, ok := s.pos[opens]; !ok || opens == gate {
		return
	}
	if s.isMember(gate, src.ID) {
		s.recordEdge(gate, opens, src.ID, b, parent)
		return
	}
	if !s.opensAfter(opens, gate) {
		return
	}
	key := src.ID + "\x00" + b
	if s.at[key] != nil {
		return
	}
	hit := &opensLaterHit{gate: gate, later: opens, target: b, from: []string{src.ID}, path: reachPath(parent, src.ID, b)}
	s.at[key] = hit
	s.warns = append(s.warns, hit)
}

// recordEdge notes member reaching b on the edge from gate to opens: the
// edge's first target names it, and each further member reaching that
// target joins it.
func (s *opensLaterScan) recordEdge(gate, opens, member, b string, parent map[string]string) {
	for _, e := range s.from[gate] {
		if e.opens != opens {
			continue
		}
		if e.target == b && !slices.Contains(e.members, member) {
			e.members = append(e.members, member)
		}
		return
	}
	e := &waitEdge{gate: gate, opens: opens, target: b, members: []string{member}, path: reachPath(parent, member, b)}
	s.from[gate] = append(s.from[gate], e)
	s.edges = append(s.edges, e)
}

// isMember reports whether id is an effective member of milestone m's gate.
func (s *opensLaterScan) isMember(m, id string) bool {
	if s.members[m] == nil {
		members, _ := GateMembers(s.g, m)
		s.members[m] = map[string]bool{}
		for _, x := range members {
			s.members[m][x] = true
		}
	}
	return s.members[m][id]
}

// cycleThrough returns the shortest cycle of the wait graph through first,
// as its gate-to-opening edges in order, each one's opening waiting on the
// next one's gate; nil when first lies on none.
func (s *opensLaterScan) cycleThrough(first *waitEdge) []*waitEdge {
	prev := map[*waitEdge]*waitEdge{}
	reached := map[string]bool{first.gate: true}
	queue := []*waitEdge{first}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, m := range s.milestones {
			if !s.opensAfter(cur.opens, m) {
				continue
			}
			if m == first.gate {
				return edgeChain(prev, first, cur)
			}
			if reached[m] {
				continue
			}
			reached[m] = true
			for _, next := range s.from[m] {
				prev[next] = cur
				queue = append(queue, next)
			}
		}
	}
	return nil
}

// edgeChain is the walk cycleThrough found from first to last, in order.
func edgeChain(prev map[*waitEdge]*waitEdge, first, last *waitEdge) []*waitEdge {
	chain := []*waitEdge{last}
	for cur := last; cur != first; {
		cur = prev[cur]
		chain = append(chain, cur)
	}
	slices.Reverse(chain)
	return chain
}

// canonicalCycle rotates cycle to start at the edge whose rotation reads
// first, and returns it with that rotation's key.
func canonicalCycle(cycle []*waitEdge) (rotated []*waitEdge, key string) {
	best := -1
	for i := range cycle {
		var parts []string
		for j := range cycle {
			e := cycle[(i+j)%len(cycle)]
			parts = append(parts, e.gate, e.opens)
		}
		if k := strings.Join(parts, "\x00"); best < 0 || k < key {
			best, key = i, k
		}
	}
	return append(slices.Clone(cycle[best:]), cycle[:best]...), key
}

// findings turns the scan into findings: one error per wait cycle, then the
// warnings on a target no error names.
func (s *opensLaterScan) findings() []EpicLintFinding {
	var out []EpicLintFinding
	held := map[string]bool{}
	seen := map[string]bool{}
	for _, e := range s.edges {
		cycle := s.cycleThrough(e)
		if cycle == nil {
			continue
		}
		cycle, key := canonicalCycle(cycle)
		if seen[key] {
			continue
		}
		seen[key] = true
		for _, c := range cycle {
			held[c.target] = true
		}
		out = append(out, waitCycleFinding(cycle))
	}
	for _, h := range s.warns {
		if held[h.target] {
			continue
		}
		f := epicWarning(EpicRuleDependencyOpensLater, fmt.Sprintf(
			"%s in %q reaches %s in %q, which opens after %q, through %s: %s is no member of the gate of %q, so it can still finish once %q opens",
			h.from[0], h.gate, h.target, h.later, h.gate, h.path, h.from[0], h.gate, h.later), h.from[0], h.target)
		f.Names = []string{h.gate, h.later}
		out = append(out, f)
	}
	return out
}

// waitCycleFinding is the dependency-opens-later error on cycle: the
// milestones in order and, per edge, the members, the epic they reach and
// the first member's path. Gate and TargetAt name the first edge, the one
// the refusal hint offers to break.
func waitCycleFinding(cycle []*waitEdge) EpicLintFinding {
	ring := make([]string, 0, 2*len(cycle)+1)
	steps := make([]string, 0, len(cycle))
	var names, epics []string
	for i, e := range cycle {
		next := cycle[(i+1)%len(cycle)]
		ring = append(ring, fmt.Sprintf("%q", e.gate), fmt.Sprintf("%q", e.opens))
		who := "its member " + e.members[0] + " reaching " + e.target
		if len(e.members) > 1 {
			who = "its members " + strings.Join(e.members, ", ") + " reaching " + e.target + ", " + e.members[0]
		}
		steps = append(steps, fmt.Sprintf("the gate of %q waits on %q opening, %s through %s; %q opens after %q",
			e.gate, e.opens, who, e.path, e.opens, next.gate))
		names = append(names, e.gate, e.opens)
		epics = append(append(epics, e.target), e.members...)
	}
	ring = append(ring, fmt.Sprintf("%q", cycle[0].gate))
	f := epicNamedError(EpicRuleDependencyOpensLater, fmt.Sprintf(
		"the milestones wait on each other in a cycle, %s, so no gate on it can go green: %s",
		strings.Join(ring, " -> "), strings.Join(steps, "; ")), slices.Compact(sortedCopy(names)), epics...)
	f.Gate, f.TargetAt = cycle[0].gate, int32(slices.Index(f.Epics, cycle[0].target))
	return f
}

// Target is the epic a dependency-opens-later error's first wait edge
// reaches, the one its refusal hint offers to move.
func (f *EpicLintFinding) Target() string {
	return f.Epics[f.TargetAt]
}

// sortedCopy is ids sorted, in a new slice.
func sortedCopy(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

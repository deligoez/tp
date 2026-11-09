package engine

import (
	"fmt"
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

// opensLaterHit is one dependency-opens-later finding being built: epics of
// milestone gate reaching target, of the milestone later that opens after
// gate. from lists the reaching epics in the order found, path the first
// one's.
type opensLaterHit struct {
	gate, later, target string
	from                []string
	path                string
}

// opensLaterScan holds what lintEpicDependencyOpensLater reads and the
// findings it has found so far, keyed by gate or epic and target.
type opensLaterScan struct {
	g       *model.EpicGraph
	pos     map[string]milestonePos
	byID    map[string]string
	after   map[string][]string
	anc     map[string]map[string]bool
	members map[string]map[string]bool
	errs    []*opensLaterHit
	warns   []*opensLaterHit
	at      map[string]*opensLaterHit
}

// lintEpicDependencyOpensLater reports, for each epic b of a milestone B,
// each milestone A that B opens after, transitively over the effective
// opens_after, whose gate holds a member reaching b through depends_on: an
// error, since that gate can never go green before B opens. When no member
// of any such gate reaches b but another epic of such a milestone does, that
// epic is a warning: it can still finish once B opens. One finding per gate
// or epic and b. An edge later-milestone-dependency reports, to a later
// milestone of the same track, is that rule's and is not followed.
func lintEpicDependencyOpensLater(g *model.EpicGraph) []EpicLintFinding {
	s := &opensLaterScan{
		g: g, pos: milestonePositions(g), byID: epicMilestones(g), after: EffectiveOpensAfter(g),
		anc: map[string]map[string]bool{}, members: map[string]map[string]bool{}, at: map[string]*opensLaterHit{},
	}
	adj := depEdges(g, s.keepEdge)
	byID := epicsByID(g)
	for i := range g.Epics {
		e := &g.Epics[i]
		if _, tracked := s.pos[e.Milestone]; tracked && byID[e.ID] == e {
			order, parent := reachFrom(e.ID, adj, func(string) bool { return true })
			for _, b := range order {
				s.record(e, b, parent)
			}
		}
	}
	return s.findings()
}

// keepEdge leaves out the edges later-milestone-dependency reports.
func (s *opensLaterScan) keepEdge(e *model.Epic, target string) bool {
	from, ok := s.pos[e.Milestone]
	to, placed := s.pos[s.byID[target]]
	return !ok || !placed || to.track != from.track || to.index <= from.index
}

// record notes src reaching b when b's milestone opens after src's.
func (s *opensLaterScan) record(src *model.Epic, b string, parent map[string]string) {
	gate, later := src.Milestone, s.byID[b]
	if _, ok := s.pos[later]; !ok || later == gate {
		return
	}
	if s.anc[later] == nil {
		s.anc[later] = opensAfterClosure(s.after, later)
	}
	if !s.anc[later][gate] {
		return
	}
	member := s.isMember(gate, src.ID)
	key := "w\x00" + src.ID + "\x00" + b
	if member {
		key = "e\x00" + gate + "\x00" + b
	}
	if hit := s.at[key]; hit != nil {
		hit.from = append(hit.from, src.ID)
		return
	}
	hit := &opensLaterHit{gate: gate, later: later, target: b, from: []string{src.ID}, path: reachPath(parent, src.ID, b)}
	s.at[key] = hit
	if member {
		s.errs = append(s.errs, hit)
	} else {
		s.warns = append(s.warns, hit)
	}
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

// findings turns the hits into findings: the errors, then the warnings on a
// target no error names.
func (s *opensLaterScan) findings() []EpicLintFinding {
	out := make([]EpicLintFinding, 0, len(s.errs)+len(s.warns))
	held := map[string]bool{}
	for _, h := range s.errs {
		held[h.target] = true
		who := "member " + h.from[0] + " reaches " + h.target + fmt.Sprintf(" in %q", h.later)
		if len(h.from) > 1 {
			who = "members " + strings.Join(h.from, ", ") + " reach " + h.target + fmt.Sprintf(" in %q, ", h.later) + h.from[0]
		}
		f := epicNamedError(EpicRuleDependencyOpensLater, fmt.Sprintf(
			"the gate of %q can never go green before %q opens, which opens after %q: its %s through %s",
			h.gate, h.later, h.gate, who, h.path),
			[]string{h.gate, h.later}, append([]string{h.target}, h.from...)...)
		f.Gate = h.gate
		out = append(out, f)
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

package engine

import (
	"fmt"
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// lintEpicDeferrableRequired reports each deferrable epic without a done
// record that an effective member of its gate without one reaches through
// plain edges and epics without one: the gate cannot go green without it,
// so the deferral changes nothing. An any-of alternative is not followed,
// since choosing the other option leaves the deferrable free, and an epic
// the gate lists as both member and deferrable is gate-member-deferrable's.
// The lint reads no task file, so done is the graph's done record alone,
// where tp epic scope's deferrable_in_closure reads derived states. One
// warning per gate and deferrable, in track and deferrable order, naming
// every direct requirer: each epic of the required set (the undone members
// and the undone epics they so reach) whose own plain edge names the
// deferrable, in graph order, capped (CappedList), since fixing one path
// leaves the warning standing while another requirer remains; and, as an
// example, the first member reaching it, in gate order, with its shortest
// path.
func lintEpicDeferrableRequired(g *model.EpicGraph) []EpicLintFinding {
	byID := epicsByID(g)
	undone := func(id string) bool { return byID[id] != nil && byID[id].Done == nil }
	adj := depEdges(g, func(*model.Epic, string) bool { return true })
	var out []EpicLintFinding
	for _, m := range trackedMilestones(g) {
		members, deferrable := GateMembers(g, m)
		req := requiredSet(members, adj, undone)
		seen := map[string]bool{}
		for _, d := range deferrable {
			p, ok := req.paths[d]
			if !ok || seen[d] || slices.Contains(members, d) {
				continue
			}
			seen[d] = true
			requirers := req.requirersOf(g, d, adj)
			f := epicWarning(EpicRuleDeferrableRequired, fmt.Sprintf(
				"deferrable %s of the gate of %q is required by %s: for example the member %s reaches it through %s, so the gate cannot go green without it",
				d, m, CappedList(requirers), p.member, p.path), append([]string{d, p.member}, requirers...)...)
			f.Names = []string{m}
			out = append(out, f)
		}
	}
	return out
}

// memberPath is the first gate member reaching an epic and its path.
type memberPath struct {
	member, path string
}

// requiredEpics is the required set of one gate: every undone member and
// every undone epic one reaches through plain edges and undone epics, each
// with the first member, in members order, reaching it.
type requiredEpics struct {
	in    map[string]bool
	paths map[string]memberPath
}

// requiredSet walks plain edges from each undone member through undone epics.
func requiredSet(members []string, adj map[string][]string, undone func(string) bool) requiredEpics {
	req := requiredEpics{in: map[string]bool{}, paths: map[string]memberPath{}}
	for _, id := range members {
		if !undone(id) {
			continue
		}
		req.in[id] = true
		order, parent := reachFrom(id, adj, undone)
		for _, x := range order {
			if _, found := req.paths[x]; found || !undone(x) || x == id {
				continue
			}
			req.in[x] = true
			req.paths[x] = memberPath{member: id, path: reachPath(parent, id, x)}
		}
	}
	return req
}

// requirersOf lists, in graph order, the epics of the required set other
// than d whose own plain edge names d.
func (r *requiredEpics) requirersOf(g *model.EpicGraph, d string, adj map[string][]string) []string {
	var out []string
	for i := range g.Epics {
		id := g.Epics[i].ID
		if id != d && r.in[id] && slices.Contains(adj[id], d) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

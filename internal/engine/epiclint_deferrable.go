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
// warning per gate and deferrable, in track and deferrable order, naming the
// first member reaching it, in gate order, and its shortest path.
func lintEpicDeferrableRequired(g *model.EpicGraph) []EpicLintFinding {
	byID := epicsByID(g)
	undone := func(id string) bool { return byID[id] != nil && byID[id].Done == nil }
	adj := depEdges(g, func(*model.Epic, string) bool { return true })
	var out []EpicLintFinding
	for _, m := range trackedMilestones(g) {
		members, deferrable := GateMembers(g, m)
		paths := deferrablePaths(members, deferrable, adj, undone)
		for _, d := range deferrable {
			p, ok := paths[d]
			if !ok {
				continue
			}
			f := epicWarning(EpicRuleDeferrableRequired, fmt.Sprintf(
				"deferrable %s of the gate of %q is required: the member %s reaches it through %s, so the gate cannot go green without it",
				d, m, p.member, p.path), d, p.member)
			f.Names = []string{m}
			out = append(out, f)
			delete(paths, d)
		}
	}
	return out
}

// memberPath is the first gate member reaching a deferrable and its path.
type memberPath struct {
	member, path string
}

// deferrablePaths maps each undone deferrable that is no member to the first
// undone member, in members order, reaching it through undone epics.
func deferrablePaths(members, deferrable []string, adj map[string][]string, undone func(string) bool) map[string]memberPath {
	out := map[string]memberPath{}
	for _, id := range members {
		if !undone(id) {
			continue
		}
		_, parent := reachFrom(id, adj, undone)
		for _, d := range deferrable {
			if _, found := out[d]; found || slices.Contains(members, d) || !undone(d) {
				continue
			}
			if _, reached := parent[d]; reached {
				out[d] = memberPath{member: id, path: reachPath(parent, id, d)}
			}
		}
	}
	return out
}

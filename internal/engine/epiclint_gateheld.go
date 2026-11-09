package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// lintEpicGateHeldByCondition reports each effective gate member without a
// done record that reaches a condition without one through epics without one:
// the operator's decision holds the gate. One info finding per member and
// condition, naming the first milestone, in track order, whose gate lists the
// member, and the shortest path. The lint reads no task file, so a work epic
// counts as done only through its done record.
func lintEpicGateHeldByCondition(g *model.EpicGraph) []EpicLintFinding {
	byID := epicsByID(g)
	undone := func(id string) bool { return byID[id].Done == nil }
	adj := depEdges(g, func(*model.Epic, string) bool { return true })
	seen := map[string]bool{}
	var out []EpicLintFinding
	for _, track := range g.Milestones {
		for _, m := range track {
			members, _ := GateMembers(g, m)
			for _, id := range members {
				if seen[id] || byID[id] == nil || !undone(id) {
					continue
				}
				seen[id] = true
				order, parent := reachFrom(id, adj, undone)
				for _, c := range order {
					if c != id && byID[c].Condition && undone(c) {
						out = append(out, gateHeldFinding(m, id, c, reachPath(parent, id, c)))
					}
				}
			}
		}
	}
	return out
}

// gateHeldFinding is the gate-held-by-condition finding on member of
// milestone m's gate waiting on condition along path.
func gateHeldFinding(m, member, condition, path string) EpicLintFinding {
	f := epicWarning(EpicRuleGateHeldByCondition, fmt.Sprintf(
		"gate member %s of %q waits on the undone condition %s through %s", member, m, condition, path), member, condition)
	f.Severity = EpicSeverityInfo
	f.Names = []string{m}
	return f
}

package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// gateHeld is one gate-held-by-condition finding being gathered: the members
// of milestone's gate that reach condition, in gate order, and the path from
// the first of them.
type gateHeld struct {
	milestone, condition, path string
	members                    []string
}

// lintEpicGateHeldByCondition reports each condition without a done record
// that effective gate members without one reach through epics without one:
// the operator's decision holds the gate. A condition in the gate's own
// milestone is skipped, since a trigger-led milestone waiting on its own
// trigger is the intended shape. One info finding per gate milestone and
// condition, listing every member that reaches it and the shortest path from
// the first; a member counts for the first milestone, in track order, whose
// gate lists it. The lint reads no task file, so a work epic counts as done
// only through its done record.
func lintEpicGateHeldByCondition(g *model.EpicGraph) []EpicLintFinding {
	byID := epicsByID(g)
	undone := func(id string) bool { return byID[id].Done == nil }
	adj := depEdges(g, func(*model.Epic, string) bool { return true })
	seen := map[string]bool{}
	var out []EpicLintFinding
	for _, track := range g.Milestones {
		for _, m := range track {
			members, _ := GateMembers(g, m)
			var fresh []string
			for _, id := range members {
				if !seen[id] && byID[id] != nil && undone(id) {
					seen[id] = true
					fresh = append(fresh, id)
				}
			}
			for _, h := range gateHeldIn(m, fresh, byID, adj, undone) {
				out = append(out, gateHeldFinding(h))
			}
		}
	}
	return out
}

// gateHeldIn groups, per undone condition outside milestone m, the members of
// m's gate that reach it through undone epics, in order of first reach.
func gateHeldIn(m string, members []string, byID map[string]*model.Epic, adj map[string][]string, undone func(string) bool) []*gateHeld {
	var held []*gateHeld
	index := map[string]*gateHeld{}
	for _, id := range members {
		order, parent := reachFrom(id, adj, undone)
		for _, c := range order {
			if c == id || !byID[c].Condition || !undone(c) || byID[c].Milestone == m {
				continue
			}
			if index[c] == nil {
				index[c] = &gateHeld{milestone: m, condition: c, path: reachPath(parent, id, c)}
				held = append(held, index[c])
			}
			index[c].members = append(index[c].members, id)
		}
	}
	return held
}

// gateHeldFinding is the gate-held-by-condition finding on h's members of
// h's milestone's gate waiting on h's condition, with h's example path.
func gateHeldFinding(h *gateHeld) EpicLintFinding {
	subject := fmt.Sprintf("gate member %s of %q waits", h.members[0], h.milestone)
	via := "through"
	if len(h.members) > 1 {
		subject = fmt.Sprintf("gate members %s of %q wait", strings.Join(h.members, ", "), h.milestone)
		via = "for example through"
	}
	f := epicWarning(EpicRuleGateHeldByCondition, fmt.Sprintf(
		"%s on the undone condition %s %s %s", subject, h.condition, via, h.path), append([]string{h.condition}, h.members...)...)
	f.Severity = EpicSeverityInfo
	f.Names = []string{h.milestone}
	return f
}

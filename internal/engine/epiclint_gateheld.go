package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// gateHeld is one gate-held-by-condition finding being gathered: the members
// of milestone's gate that reach the undone condition, or the any-of group
// of them, label words, in gate order, and the path from the first of them.
type gateHeld struct {
	milestone, label, path string
	conditions, members    []string
}

// lintEpicGateHeldByCondition reports each condition without a done record
// that effective gate members without one reach through epics without one:
// the operator's decision holds the gate. A condition in the gate's own
// milestone is skipped, since a trigger-led milestone waiting on its own
// trigger is the intended shape. An any-of group counts only when every
// member is such a condition (spec/1.7.0.md §7). One info finding per gate
// milestone and condition or group, listing every member that reaches it and
// the shortest path from the first; a member counts for the first milestone,
// in track order, whose gate lists it. The lint reads no task file, so a
// work epic counts as done only through its done record.
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
			s := &gateHeldScan{m: m, byID: byID, undone: undone, index: map[string]*gateHeld{}}
			for _, h := range s.scan(fresh, adj) {
				out = append(out, gateHeldFinding(h))
			}
		}
	}
	return out
}

// gateHeldScan gathers the gate-held-by-condition findings of milestone m's
// gate, keyed by condition or group member set, in order of first reach.
type gateHeldScan struct {
	m      string
	byID   map[string]*model.Epic
	undone func(string) bool
	held   []*gateHeld
	index  map[string]*gateHeld
}

// scan groups, per undone condition outside m and per group of them, the
// members of m's gate that reach it through undone epics.
func (s *gateHeldScan) scan(members []string, adj map[string][]string) []*gateHeld {
	for _, id := range members {
		order, parent := reachFrom(id, adj, s.undone)
		s.groupsOf(id, id, id)
		for _, c := range order {
			if c == id || !s.undone(c) {
				continue
			}
			path := reachPath(parent, id, c)
			if e := s.byID[c]; e.Condition && e.Milestone != s.m {
				s.note(c, fmt.Sprintf("the undone condition %s", c), path, []string{c}, id)
			}
			s.groupsOf(c, id, path)
		}
	}
	return s.held
}

// groupsOf notes each group of the epic id that holds member's gate on the
// operator alone: every member an undone condition outside m. path leads
// from member to id.
func (s *gateHeldScan) groupsOf(id, member, path string) {
	for _, d := range s.byID[id].DependsOn {
		if !d.IsGroup() || !s.allUndoneConditionsOutside(d.Members()) {
			continue
		}
		label := epicDepLabel(d)
		s.note("\x00"+strings.Join(d.MemberSet(), "\x00"), label+", each an undone condition,", path+" -> "+label, d.Members(), member)
	}
}

// allUndoneConditionsOutside reports whether ids holds two or more epics,
// each an undone condition outside m.
func (s *gateHeldScan) allUndoneConditionsOutside(ids []string) bool {
	if len(ids) < 2 {
		return false
	}
	for _, id := range ids {
		e := s.byID[id]
		if e == nil || !e.Condition || !s.undone(id) || e.Milestone == s.m {
			return false
		}
	}
	return true
}

// note adds member to the finding keyed key, opening it with label, path
// and conditions when member is the first to reach it.
func (s *gateHeldScan) note(key, label, path string, conditions []string, member string) {
	h := s.index[key]
	if h == nil {
		h = &gateHeld{milestone: s.m, label: label, path: path, conditions: conditions}
		s.index[key] = h
		s.held = append(s.held, h)
	}
	if !slices.Contains(h.members, member) {
		h.members = append(h.members, member)
	}
}

// gateHeldFinding is the gate-held-by-condition finding on h's members of
// h's milestone's gate waiting on h's condition or group, with h's example
// path.
func gateHeldFinding(h *gateHeld) EpicLintFinding {
	subject := fmt.Sprintf("gate member %s of %q waits", h.members[0], h.milestone)
	via := "through"
	if len(h.members) > 1 {
		subject = fmt.Sprintf("gate members %s of %q wait", strings.Join(h.members, ", "), h.milestone)
		via = "for example through"
	}
	f := epicWarning(EpicRuleGateHeldByCondition, fmt.Sprintf(
		"%s on %s %s %s", subject, h.label, via, h.path), append(slices.Clone(h.conditions), h.members...)...)
	f.Severity = EpicSeverityInfo
	f.Names = []string{h.milestone}
	return f
}

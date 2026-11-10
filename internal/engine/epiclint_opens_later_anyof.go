package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// groupWait is an unsatisfied any-of group that members of milestone gate's
// gate reach through depends_on and undone epics: d is the group, members
// the gate members reaching it in the order found, path the first one's.
type groupWait struct {
	gate, path string
	d          model.EpicDep
	members    []string
}

// noteGroups notes each unsatisfied group that src, an undone epic, reaches
// through the plain edges order and parent describe, itself included, when
// src is a member of its milestone's gate.
func (s *opensLaterScan) noteGroups(src *model.Epic, order []string, parent map[string]string) {
	if !s.isMember(src.Milestone, src.ID) {
		return
	}
	s.noteGroupsOf(src, src.ID, src.ID)
	for _, w := range order {
		if w != src.ID && s.epics[w].Done == nil {
			s.noteGroupsOf(src, w, reachPath(parent, src.ID, w))
		}
	}
}

// noteGroupsOf notes each unsatisfied group of the epic w that src reaches
// through path.
func (s *opensLaterScan) noteGroupsOf(src *model.Epic, w, path string) {
	for _, d := range s.epics[w].DependsOn {
		if !d.IsGroup() || len(d.Members()) == 0 || s.groupSatisfied(d) {
			continue
		}
		key := src.Milestone + "\x00" + strings.Join(d.MemberSet(), "\x00")
		if c := s.groupAt[key]; c != nil {
			if !slices.Contains(c.members, src.ID) {
				c.members = append(c.members, src.ID)
			}
			continue
		}
		c := &groupWait{gate: src.Milestone, path: path + " -> " + epicDepLabel(d), d: d, members: []string{src.ID}}
		s.groupAt[key] = c
		s.groups = append(s.groups, c)
	}
}

// groupSatisfied reports whether a member of d has a done record.
func (s *opensLaterScan) groupSatisfied(d model.EpicDep) bool {
	return slices.ContainsFunc(d.Members(), func(id string) bool {
		e := s.epics[id]
		return e != nil && e.Done != nil
	})
}

// groupWaitFindings is one error per noted group whose every member waits
// on its gate (gateWaits), so no member can be done before that gate goes
// green (spec/1.7.0.md §7); it marks each member held.
func (s *opensLaterScan) groupWaitFindings(held map[string]bool) []EpicLintFinding {
	var out []EpicLintFinding
	for _, c := range s.groups {
		waits := s.gateWaits(c.gate)
		members := c.d.Members()
		if slices.ContainsFunc(members, func(id string) bool { return waits[id] == "" }) {
			continue
		}
		for _, id := range members {
			held[id] = true
		}
		out = append(out, groupWaitFinding(c, waits))
	}
	return out
}

// gateWaits returns, for milestone gate, each undone epic that cannot be
// done before gate's gate goes green, mapped to the milestone whose opening
// it waits on. It is the least fixed point over the wait graph: an epic
// waits when its milestone opens after a gate that waits, when a plain edge
// keepEdge keeps leads to an epic that waits, or when every member of an
// unsatisfied group of it waits, each through its own opening; a gate
// waits when one of its members in its milestone does. Group members are
// never filtered by keepEdge: a member in a later milestone of the same
// track is legal while another member is not.
func (s *opensLaterScan) gateWaits(gate string) map[string]string {
	if s.waitsAt[gate] != nil {
		return s.waitsAt[gate]
	}
	gates := map[string]bool{gate: true}
	waits := map[string]string{}
	for grew := true; grew; {
		grew = false
		for i := range s.g.Epics {
			e := &s.g.Epics[i]
			if s.epics[e.ID] != e || e.Done != nil || waits[e.ID] != "" {
				continue
			}
			if m := s.epicWaits(e, gates, waits); m != "" {
				waits[e.ID], grew = m, true
				if s.isMember(e.Milestone, e.ID) {
					gates[e.Milestone] = true
				}
			}
		}
	}
	s.waitsAt[gate] = waits
	return waits
}

// epicWaits returns the milestone whose opening e waits on, given the gates
// and epics found waiting so far, or "" when e waits on none yet.
func (s *opensLaterScan) epicWaits(e *model.Epic, gates map[string]bool, waits map[string]string) string {
	if _, tracked := s.pos[e.Milestone]; tracked {
		for _, m := range s.milestones {
			if gates[m] && s.opensAfter(e.Milestone, m) {
				return e.Milestone
			}
		}
	}
	for _, d := range e.DependsOn {
		if w := s.depWaits(e, d, waits); w != "" {
			return w
		}
	}
	return ""
}

// depWaits returns the opening e's entry d makes it wait on: a kept plain
// edge's epic's, or, when every member of a group waits, its first
// member's; "" otherwise.
func (s *opensLaterScan) depWaits(e *model.Epic, d model.EpicDep, waits map[string]string) string {
	if !d.IsGroup() {
		if s.epics[d.ID] == nil || !s.keepEdge(e, d.ID) {
			return ""
		}
		return waits[d.ID]
	}
	members := d.Members()
	if len(members) == 0 || slices.ContainsFunc(members, func(id string) bool { return waits[id] == "" }) {
		return ""
	}
	return waits[members[0]]
}

// groupWaitFinding is the dependency-opens-later error on c: the gate, the
// group, the members reaching it and the first one's path, and the opening
// each group member waits on. Gate and TargetAt name the gate and the
// group's first member, the one the refusal hint offers to move.
func groupWaitFinding(c *groupWait, waits map[string]string) EpicLintFinding {
	members := c.d.Members()
	steps := make([]string, 0, len(members))
	names := []string{c.gate}
	for _, id := range members {
		steps = append(steps, fmt.Sprintf("%s waits on %q opening", id, waits[id]))
		names = append(names, waits[id])
	}
	who := "its member " + c.members[0] + " reaching it through " + c.path
	if len(c.members) > 1 {
		who = "its members " + strings.Join(c.members, ", ") + " reaching it, " + c.members[0] + " through " + c.path
	}
	f := epicNamedError(EpicRuleDependencyOpensLater, fmt.Sprintf(
		"the gate of %q waits on %s, %s, and no member can be done before that gate goes green: %s, and each of those openings waits on the gate of %q",
		c.gate, epicDepLabel(c.d), who, strings.Join(steps, "; "), c.gate),
		slices.Compact(sortedCopy(names)), append(slices.Clone(c.members), members...)...)
	f.Gate, f.TargetAt = c.gate, int32(slices.Index(f.Epics, members[0]))
	return f
}

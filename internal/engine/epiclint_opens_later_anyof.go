package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// groupWait is an unsatisfied any-of group that members of milestone gate's
// gate reach through depends_on and undone epics: d is the group, members
// the gate members reaching it in the order found, path the first one's,
// holder the epic whose entry it is.
type groupWait struct {
	gate, path, holder string
	d                  model.EpicDep
	members            []string
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
		c := &groupWait{gate: src.Milestone, path: path + " -> " + epicDepLabel(d), holder: w, d: d, members: []string{src.ID}}
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
// green (spec/1.7.0.md §7), once per epic and member set; it marks each
// member held.
func (s *opensLaterScan) groupWaitFindings(held map[string]bool) []EpicLintFinding {
	var out []EpicLintFinding
	reported := map[string]bool{}
	for _, c := range s.groups {
		key := c.holder + "\x00" + strings.Join(c.d.MemberSet(), "\x00")
		if reported[key] {
			continue
		}
		waits := s.gateWaits(c.gate)
		members := c.d.Members()
		if slices.ContainsFunc(members, func(id string) bool { _, ok := waits[id]; return !ok }) {
			continue
		}
		reported[key] = true
		for _, id := range members {
			held[id] = true
		}
		out = append(out, groupWaitFinding(c, waits))
	}
	return out
}

// gateWaits returns, for milestone gate, each undone epic that cannot be
// done while gate's gate is not green, mapped to the milestone whose
// opening it waits on ("" when it waits only on epics that wait). It is the
// complement of the least fixed point of what can be done with that gate
// never green (doableWithout), so members that each wait on the gate only
// through the other, as far as the wait graph reaches, wait too. Group
// members are never filtered by keepEdge: a member in a later milestone of
// the same track is legal while another member is not.
func (s *opensLaterScan) gateWaits(gate string) map[string]string {
	if waits, ok := s.waitsAt[gate]; ok {
		return waits
	}
	can, green := s.doableWithout(gate)
	waits := map[string]string{}
	for i := range s.g.Epics {
		e := &s.g.Epics[i]
		if s.epics[e.ID] == e && !can[e.ID] {
			waits[e.ID] = s.waitWitness(e, can, green, map[string]bool{})
		}
	}
	s.waitsAt[gate] = waits
	return waits
}

// doableWithout grows, from the epics with a done record, the epics that
// can be done and the gates that can go green while gate's never does: an
// epic once its milestone's opening waits on no gate that is not green and
// each depends_on entry can be met (canDo); a gate once every member it
// lists that the graph holds can be done.
func (s *opensLaterScan) doableWithout(gate string) (can, green map[string]bool) {
	can, green = map[string]bool{}, map[string]bool{}
	for id, e := range s.epics {
		can[id] = e.Done != nil
	}
	for grew := true; grew; {
		grew = false
		for _, m := range s.milestones {
			if m != gate && !green[m] && s.gateCanGoGreen(m, can) {
				green[m], grew = true, true
			}
		}
		for i := range s.g.Epics {
			e := &s.g.Epics[i]
			if s.epics[e.ID] == e && !can[e.ID] && s.openingCanOpen(e, green) && s.entriesCanBeMet(e, can) {
				can[e.ID], grew = true, true
			}
		}
	}
	return can, green
}

// gateCanGoGreen reports whether every effective member of milestone m's
// gate that the graph holds can be done.
func (s *opensLaterScan) gateCanGoGreen(m string, can map[string]bool) bool {
	s.isMember(m, "")
	for id := range s.members[m] {
		if s.epics[id] != nil && !can[id] {
			return false
		}
	}
	return true
}

// openingCanOpen reports whether e's milestone opens after no tracked
// milestone whose gate is not green; an untracked milestone always can.
func (s *opensLaterScan) openingCanOpen(e *model.Epic, green map[string]bool) bool {
	if _, tracked := s.pos[e.Milestone]; !tracked {
		return true
	}
	return !slices.ContainsFunc(s.milestones, func(m string) bool { return !green[m] && s.opensAfter(e.Milestone, m) })
}

// entriesCanBeMet reports whether each depends_on entry of e can be met: a
// kept plain edge to an epic that can be done, a group with a member that
// can. An edge to no epic, an edge keepEdge drops and a group naming no epic
// are other rules' errors and hold nothing here.
func (s *opensLaterScan) entriesCanBeMet(e *model.Epic, can map[string]bool) bool {
	for _, d := range e.DependsOn {
		if !d.IsGroup() {
			if s.epics[d.ID] != nil && s.keepEdge(e, d.ID) && !can[d.ID] {
				return false
			}
			continue
		}
		held := slices.DeleteFunc(d.Members(), func(id string) bool { return s.epics[id] == nil })
		if len(held) > 0 && !slices.ContainsFunc(held, func(id string) bool { return can[id] }) {
			return false
		}
	}
	return true
}

// waitWitness names the milestone whose opening e, which cannot be done,
// waits on: its own when that opening cannot open, else that of the first
// kept plain edge or the first member of the first group that cannot be
// met; "" when the walk only comes back to an epic seen.
func (s *opensLaterScan) waitWitness(e *model.Epic, can, green, seen map[string]bool) string {
	if !s.openingCanOpen(e, green) {
		return e.Milestone
	}
	seen[e.ID] = true
	for _, d := range e.DependsOn {
		next := ""
		switch {
		case !d.IsGroup() && s.epics[d.ID] != nil && s.keepEdge(e, d.ID) && !can[d.ID]:
			next = d.ID
		case d.IsGroup() && len(d.Members()) > 0 && s.epics[d.Members()[0]] != nil && !can[d.Members()[0]]:
			next = d.Members()[0]
		}
		if next != "" && !seen[next] {
			if w := s.waitWitness(s.epics[next], can, green, seen); w != "" {
				return w
			}
		}
	}
	return ""
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
		if waits[id] == "" {
			steps = append(steps, id+" waits only on epics that wait")
			continue
		}
		steps = append(steps, fmt.Sprintf("%s waits on %q opening", id, waits[id]))
		names = append(names, waits[id])
	}
	who := "its member " + c.members[0] + " reaching it through " + c.path
	if len(c.members) > 1 {
		who = "its members " + strings.Join(c.members, ", ") + " reaching it, " + c.members[0] + " through " + c.path
	}
	f := epicNamedError(EpicRuleDependencyOpensLater, fmt.Sprintf(
		"the gate of %q waits on %s, %s, and no member can be done before that gate goes green: %s, and each of those openings waits, as far as the wait graph reaches, on the gate of %q",
		c.gate, epicDepLabel(c.d), who, strings.Join(steps, "; "), c.gate),
		slices.Compact(sortedCopy(names)), append(slices.Clone(c.members), members...)...)
	f.Gate, f.TargetAt = c.gate, int32(slices.Index(f.Epics, members[0]))
	return f
}

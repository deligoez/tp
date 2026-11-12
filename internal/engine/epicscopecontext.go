package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicScopeContext renders milestone's live scope (MilestoneScope) as the
// context a graph role asked scope:<m> reads (spec/backlog/epic-scope.md,
// Part B): the members by mode and count, each closure and outside epic once,
// by its title and acceptance with its marks and its edges by id to the other
// epics shown, then the shrink rows, deferrable_in_closure, holds_opening and
// the critical path by count. It is live, unlike the unit's status-blind
// block, so it is context and never part of a hash.
func EpicScopeContext(g *model.EpicGraph, states map[string]EpicState, milestone string) string {
	s := MilestoneScope(g, states, nil, milestone)
	epics := epicsByID(g)
	present := map[string]bool{}
	for _, m := range s.Closure {
		for _, e := range m.Epics {
			present[e.ID] = true
		}
	}
	for _, o := range s.Outside {
		present[o.ID] = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\nScope of gate %s — the live tp epic scope measurement behind the asked scope unit; its epics are context, not units:\n", milestone)
	mode := "explicit"
	if s.MembersImplicit {
		mode = "implicit"
	}
	fmt.Fprintf(&b, "members: %s, %d\n", mode, s.MemberCount)
	b.WriteString("closure, then the milestone's epics outside it, each epic once:\n")
	for _, m := range s.Closure {
		fmt.Fprintf(&b, "--- %s%s, %d in the closure\n", m.Milestone, scopeTrackLabel(m), m.Size)
		for _, e := range m.Epics {
			writeScopeBrief(&b, epics[e.ID], scopeClosureMarks(&s, e, epics[e.ID]), present)
		}
	}
	if len(s.Outside) > 0 {
		fmt.Fprintf(&b, "--- %s, outside the closure\n", milestone)
		for _, o := range s.Outside {
			writeScopeBrief(&b, epics[o.ID], scopeOutsideMarks(o), present)
		}
	}
	writeScopeTail(&b, &s)
	return b.String()
}

// scopeTrackLabel names a closure milestone's track and cross-track mark.
func scopeTrackLabel(m ScopeMilestone) string {
	label := ""
	if m.Track != nil {
		label = fmt.Sprintf(" (track %d)", *m.Track)
	}
	if m.CrossTrack {
		label += " cross-track"
	}
	return label
}

// scopeClosureMarks lists a closure epic's marks: member, any_of, deferrable
// and condition.
func scopeClosureMarks(s *EpicScope, e ScopeEpic, epic *model.Epic) []string {
	var marks []string
	if slices.Contains(s.Members, e.ID) {
		marks = append(marks, "member")
	}
	if len(e.AnyOf) > 0 {
		marks = append(marks, "any_of: "+strings.Join(e.AnyOf, ", "))
	}
	if e.Deferrable {
		marks = append(marks, "deferrable")
	}
	if epic.Condition {
		marks = append(marks, "condition")
	}
	return marks
}

// scopeOutsideMarks lists an outside epic's marks.
func scopeOutsideMarks(o ScopeOutside) []string {
	marks := []string{"outside"}
	if o.Deferrable {
		marks = append(marks, "deferrable")
	}
	if o.Condition {
		marks = append(marks, "condition")
	}
	if o.NoDependents {
		marks = append(marks, "no_dependents")
	}
	return marks
}

// writeScopeBrief writes one epic's row: its id, marks and title, then its
// acceptance and its edges with their whys when it has them.
func writeScopeBrief(b *strings.Builder, e *model.Epic, marks []string, present map[string]bool) {
	b.WriteString("- " + e.ID + " ")
	if len(marks) > 0 {
		b.WriteString("[" + strings.Join(marks, ", ") + "] ")
	}
	b.WriteString(e.Title + "\n")
	if e.Acceptance != "" {
		b.WriteString("  acceptance: " + e.Acceptance + "\n")
	}
	var edges []string
	for _, d := range e.DependsOn {
		if !d.IsGroup() {
			if present[d.ID] {
				edges = append(edges, d.ID)
			}
			continue
		}
		if group := slices.DeleteFunc(slices.Clone(d.Members()), func(id string) bool { return !present[id] }); len(group) > 0 {
			edges = append(edges, "one of {"+strings.Join(group, ", ")+"}")
		}
	}
	if len(edges) > 0 {
		b.WriteString("  depends_on: " + strings.Join(edges, ", ") + "\n")
	}
}

// writeScopeTail writes the shrink rows, deferrable_in_closure, holds_opening
// and the critical path by count.
func writeScopeTail(b *strings.Builder, s *EpicScope) {
	b.WriteString("shrink — the epics that leave the closure when the epic is cut, or dropped from the gate when a member, largest first:\n")
	if len(s.Shrink) == 0 {
		b.WriteString("(none)\n")
	}
	for _, r := range s.Shrink {
		var marks []string
		if r.Member {
			marks = append(marks, "member")
		}
		if len(r.AnyOf) > 0 {
			marks = append(marks, "any_of: "+strings.Join(r.AnyOf, ", "))
		}
		label := r.Epic
		if len(marks) > 0 {
			label += " [" + strings.Join(marks, ", ") + "]"
		}
		fmt.Fprintf(b, "* %s size %d: %s\n", label, r.Size, strings.Join(r.Leaves, ", "))
	}
	fmt.Fprintf(b, "deferrable_in_closure: %s\n", scopeListOrNone(s.DeferrableInClosure, ", "))
	holds := make([]string, 0, len(s.HoldsOpening))
	for _, h := range s.HoldsOpening {
		holds = append(holds, h.Milestone+": "+strings.Join(h.Epics, ", "))
	}
	b.WriteString("holds_opening: " + scopeListOrNone(holds, "; ") + "\n")
	fmt.Fprintf(b, "critical_path (by count): %s\n", strings.Join(s.CriticalPath.ByCount, " -> "))
}

// scopeListOrNone joins items with sep, or says none.
func scopeListOrNone(items []string, sep string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, sep)
}

package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// lintEpicGateEmpty reports each tracked milestone holding epics whose gate
// has no effective members and no threshold while some milestone opens after
// it (EffectiveOpensAfter): that gate is green at once, so the milestones
// opening after it open without waiting on anything, a condition the
// milestone holds included. A milestone with no epics yet holds nothing that
// looks like a gate, and one whose epics are all done conditions holds a
// trigger that has fired, so neither is reported. One warning per milestone, in
// track order, epics the milestone's own epics, its message naming every
// milestone that opens after it.
func lintEpicGateEmpty(g *model.EpicGraph) []EpicLintFinding {
	after := EffectiveOpensAfter(g)
	var order []string
	seen := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			if !seen[m] {
				seen[m] = true
				order = append(order, m)
			}
		}
	}
	var out []EpicLintFinding
	for _, m := range order {
		own, fired := milestoneEpics(g, m)
		if len(own) == 0 || fired || g.Gates[m].Threshold != "" {
			continue
		}
		if members, _ := GateMembers(g, m); len(members) > 0 {
			continue
		}
		var followers []string
		for _, x := range order {
			if slices.Contains(after[x], m) {
				followers = append(followers, fmt.Sprintf("%q", x))
			}
		}
		if len(followers) == 0 {
			continue
		}
		names := strings.Join(followers, ", ")
		verb, target := "opens", "the"
		if len(followers) > 1 {
			verb, target = "open", "a"
		}
		f := epicNamedError(EpicRuleGateEmpty, fmt.Sprintf(
			"the gate of %q has no effective members and no threshold, so it is green at once and %s %s after it "+
				"without waiting; name its members (tp epic set gates.%s.members=<ids>), give it a threshold, "+
				"or move a trigger condition into %s milestone that opens after it (%s)", m, names, verb, m, target, names),
			[]string{m}, own...)
		f.Severity = EpicSeverityWarning
		out = append(out, f)
	}
	return out
}

// milestoneEpicIDs returns the ids of milestone m's epics, in graph order.
// milestoneEpics returns the ids of milestone m's epics, in graph order, and
// whether every one is a condition with a done record: a trigger that has
// fired, so the milestones opening after m are meant to be open.
func milestoneEpics(g *model.EpicGraph, m string) (ids []string, fired bool) {
	fired = true
	for i := range g.Epics {
		if e := &g.Epics[i]; e.Milestone == m {
			ids = append(ids, e.ID)
			fired = fired && e.Condition && e.Done != nil
		}
	}
	return ids, fired
}

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
// looks like a gate, so it is not reported. One warning per milestone, in
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
		own := milestoneEpicIDs(g, m)
		if len(own) == 0 || g.Gates[m].Threshold != "" {
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
		verb := "opens"
		if len(followers) > 1 {
			verb = "open"
		}
		f := epicNamedError(EpicRuleGateEmpty, fmt.Sprintf(
			"the gate of %q has no effective members and no threshold, so it is green at once and %s %s after it "+
				"without waiting; name its members (tp epic set gates.%s.members=<ids>), give it a threshold, "+
				"or move a trigger condition into the milestone it holds", m, strings.Join(followers, ", "), verb, m),
			[]string{m}, own...)
		f.Severity = EpicSeverityWarning
		out = append(out, f)
	}
	return out
}

// milestoneEpicIDs returns the ids of milestone m's epics, in graph order.
func milestoneEpicIDs(g *model.EpicGraph, m string) []string {
	var ids []string
	for i := range g.Epics {
		if g.Epics[i].Milestone == m {
			ids = append(ids, g.Epics[i].ID)
		}
	}
	return ids
}

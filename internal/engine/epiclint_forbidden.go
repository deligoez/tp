package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// lintEpicForbiddenRules reports, for each forbidden_edges rule
// (spec/1.8.0.md §10), a selector that names nothing the graph holds and a
// rule without a why. An epic, milestone or track_of selector naming nothing
// is a dangling-reference error; a tag no epic carries is a dangling-reference
// warning, so a rule may precede its tag; an empty why is missing-why.
func lintEpicForbiddenRules(g *model.EpicGraph) []EpicLintFinding {
	if len(g.ForbiddenEdges) == 0 {
		return nil
	}
	ids := epicIDs(g)
	milestones := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			milestones[m] = true
		}
	}
	tags := map[string]bool{}
	for i := range g.Epics {
		for _, tag := range g.Epics[i].Tags {
			tags[tag] = true
		}
	}
	var out []EpicLintFinding
	for i := range g.ForbiddenEdges {
		rule := &g.ForbiddenEdges[i]
		name := fmt.Sprintf("forbidden_edges[%d]", i)
		for _, side := range []struct {
			field string
			sel   model.EpicSelector
		}{{"from", rule.From}, {"to", rule.To}} {
			if f, ok := danglingSelector(name+"."+side.field, side.sel, ids, milestones, tags); ok {
				out = append(out, f)
			}
		}
		if strings.TrimSpace(rule.Why) == "" {
			f := epicNamedError(EpicRuleMissingWhy, name+" has no why", []string{name})
			f.Origin = DanglingForbiddenRule
			out = append(out, f)
		}
	}
	return out
}

// danglingSelector is the finding on a selector at path that names nothing
// the graph holds, and false when it names something.
func danglingSelector(path string, sel model.EpicSelector, ids, milestones, tags map[string]bool) (EpicLintFinding, bool) {
	var f EpicLintFinding
	switch {
	case sel.Epic != "" && !ids[sel.Epic]:
		f = epicNamedError(EpicRuleDanglingReference,
			fmt.Sprintf("%s epic %q names no epic", path, sel.Epic), []string{path}, sel.Epic)
	case sel.Milestone != "" && !milestones[sel.Milestone]:
		f = epicNamedError(EpicRuleDanglingReference,
			fmt.Sprintf("%s milestone %q names no milestone", path, sel.Milestone), []string{path, sel.Milestone})
	case sel.TrackOf != "" && !milestones[sel.TrackOf]:
		f = epicNamedError(EpicRuleDanglingReference,
			fmt.Sprintf("%s track_of %q names no milestone", path, sel.TrackOf), []string{path, sel.TrackOf})
	case sel.Tag != "" && !tags[sel.Tag]:
		f = epicNamedError(EpicRuleDanglingReference,
			fmt.Sprintf("%s tag %q is carried by no epic", path, sel.Tag), []string{path, sel.Tag})
		f.Severity = EpicSeverityWarning
	default:
		return f, false
	}
	f.Origin = DanglingForbiddenRule
	return f, true
}

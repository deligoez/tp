package engine

import (
	"fmt"
	"slices"
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

// lintEpicForbiddenDependencies reports forbidden-dependency (spec/1.8.0.md
// §10): for each rule, each direct edge and each any-of member from an epic
// its from selects to an epic its to selects, one error per edge naming the
// rule's why. Only what an epic names counts: a path through a third epic
// the rule does not select passes.
func lintEpicForbiddenDependencies(g *model.EpicGraph) []EpicLintFinding {
	if len(g.ForbiddenEdges) == 0 {
		return nil
	}
	trackOf := map[string]int{}
	for i, track := range g.Milestones {
		for _, m := range track {
			if _, ok := trackOf[m]; !ok {
				trackOf[m] = i
			}
		}
	}
	byID := map[string]*model.Epic{}
	for i := range g.Epics {
		byID[g.Epics[i].ID] = &g.Epics[i]
	}
	var out []EpicLintFinding
	for r := range g.ForbiddenEdges {
		rule := &g.ForbiddenEdges[r]
		name := fmt.Sprintf("forbidden_edges[%d]", r)
		for i := range g.Epics {
			if e := &g.Epics[i]; selectsEpic(rule.From, e, trackOf) {
				out = append(out, forbiddenEdgesOf(name, rule, e, byID, trackOf)...)
			}
		}
	}
	return out
}

// forbiddenEdgesOf is the findings on e's direct edges and any-of members
// that reach an epic the rule's to selects.
func forbiddenEdgesOf(name string, rule *model.EpicForbiddenEdge, e *model.Epic, byID map[string]*model.Epic, trackOf map[string]int) []EpicLintFinding {
	var out []EpicLintFinding
	for _, d := range e.DependsOn {
		for _, target := range d.Names() {
			if t, ok := byID[target]; ok && selectsEpic(rule.To, t, trackOf) {
				out = append(out, forbiddenDependency(name, rule.Why, e.ID, target, d.IsGroup()))
			}
		}
	}
	return out
}

// forbiddenDependency is the finding on the edge from -> to that the rule
// name forbids, through an any-of group when group.
func forbiddenDependency(name, why, from, to string, group bool) EpicLintFinding {
	via := ""
	if group {
		via = " through an any-of group"
	}
	return epicNamedError(EpicRuleForbiddenDependency,
		fmt.Sprintf("%s depends on %s%s, which %s forbids: %s", from, to, via, name, why), []string{name}, from, to)
}

// selectsEpic reports whether sel selects e: its id, its milestone, one of
// its tags, or a milestone of the track holding the named milestone.
func selectsEpic(sel model.EpicSelector, e *model.Epic, trackOf map[string]int) bool {
	switch {
	case sel.Epic != "":
		return e.ID == sel.Epic
	case sel.Milestone != "":
		return e.Milestone == sel.Milestone
	case sel.Tag != "":
		return slices.Contains(e.Tags, sel.Tag)
	case sel.TrackOf != "":
		track, named := trackOf[sel.TrackOf]
		mine, tracked := trackOf[e.Milestone]
		return named && tracked && track == mine
	}
	return false
}

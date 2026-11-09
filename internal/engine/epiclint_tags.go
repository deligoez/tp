package engine

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/deligoez/tp/internal/model"
)

func epicWarning(rule, message string, epics ...string) EpicLintFinding {
	f := epicError(rule, message, epics...)
	f.Severity = EpicSeverityWarning
	return f
}

// epicsByID maps each epic id to its first definition; a repeated id is
// duplicate-name's finding.
func epicsByID(g *model.EpicGraph) map[string]*model.Epic {
	byID := make(map[string]*model.Epic, len(g.Epics))
	for i := range g.Epics {
		if _, ok := byID[g.Epics[i].ID]; !ok {
			byID[g.Epics[i].ID] = &g.Epics[i]
		}
	}
	return byID
}

// transitiveDeps returns every epic id reachable from e over depends_on
// edges, e itself excluded unless a cycle leads back to it. Dangling targets
// are not followed.
func transitiveDeps(e *model.Epic, byID map[string]*model.Epic) map[string]bool {
	seen := map[string]bool{}
	stack := []*model.Epic{e}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range cur.DependsOn {
			if next, ok := byID[d.ID]; ok && !seen[d.ID] {
				seen[d.ID] = true
				stack = append(stack, next)
			}
		}
	}
	return seen
}

// lintEpicTagChecks reports each epic carrying a checked tag whose provider
// is not among its transitive dependencies. The provider is exempt, and a
// provider naming no epic is dangling-reference's finding.
func lintEpicTagChecks(g *model.EpicGraph) []EpicLintFinding {
	if len(g.TagChecks) == 0 {
		return nil
	}
	byID := epicsByID(g)
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		var deps map[string]bool
		for _, tag := range e.Tags {
			check, ok := g.TagChecks[tag]
			if _, exists := byID[check.ProvidedBy]; !ok || !exists || check.ProvidedBy == e.ID {
				continue
			}
			if deps == nil {
				deps = transitiveDeps(e, byID)
			}
			if !deps[check.ProvidedBy] {
				out = append(out, epicNamedError(EpicRuleUnprovidedTagCheck,
					fmt.Sprintf("epic %q carries tag %q but does not depend on its provider %q", e.ID, tag, check.ProvidedBy),
					[]string{tag}, e.ID))
			}
		}
	}
	return out
}

// lintEpicNoDependencies warns on each epic outside the first milestone of
// its track that has no dependencies. An unknown milestone is
// unknown-milestone's finding.
func lintEpicNoDependencies(g *model.EpicGraph) []EpicLintFinding {
	pos := milestonePositions(g)
	var out []EpicLintFinding
	for _, e := range g.Epics {
		if p, ok := pos[e.Milestone]; ok && p.index > 0 && len(e.DependsOn) == 0 && !e.Root {
			out = append(out, epicWarning(EpicRuleNoDependencies,
				fmt.Sprintf("epic %q in %q has no dependencies", e.ID, e.Milestone), e.ID))
		}
	}
	return out
}

// lintEpicRootWithDependencies warns on each epic marked root that has
// dependencies, so the mark does not outlive the reason for it.
func lintEpicRootWithDependencies(g *model.EpicGraph) []EpicLintFinding {
	var out []EpicLintFinding
	for _, e := range g.Epics {
		if e.Root && len(e.DependsOn) > 0 {
			out = append(out, epicWarning(EpicRuleRootWithDependencies,
				fmt.Sprintf("epic %q is marked root and has %d dependencies", e.ID, len(e.DependsOn)), e.ID))
		}
	}
	return out
}

// lintEpicThinWhy warns on each edge whose why, case-folded and split into
// words, appears as a run of whole words in the target epic's id or title. An
// empty why is missing-why's finding and a dangling target is
// dangling-reference's.
func lintEpicThinWhy(g *model.EpicGraph) []EpicLintFinding {
	byID := epicsByID(g)
	var out []EpicLintFinding
	for _, e := range g.Epics {
		for _, d := range e.DependsOn {
			target, ok := byID[d.ID]
			why := epicWords(d.Why)
			if !ok || len(why) == 0 {
				continue
			}
			if containsWordRun(epicWords(target.ID), why) || containsWordRun(epicWords(target.Title), why) {
				out = append(out, epicWarning(EpicRuleThinWhy,
					fmt.Sprintf("edge %q -> %q has a why that restates the target's id or title", e.ID, d.ID), e.ID, d.ID))
			}
		}
	}
	return out
}

// epicWords splits s, case-folded, into its runs of letters and digits.
func epicWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// containsWordRun reports whether needle occurs in hay as consecutive words.
func containsWordRun(hay, needle []string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if slices.Equal(hay[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

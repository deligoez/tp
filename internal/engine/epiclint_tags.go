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

// providerReach answers whether an epic depends on one provider
// (spec/1.7.0.md §7): through a plain edge to the provider or to an epic that
// depends on it, or through a group every member of which is the provider or
// depends on it, by the same rule at every level. Dangling ids are not
// followed, and a loop back to an epic being asked counts as not reaching:
// the loop is cycle's finding.
type providerReach struct {
	byID     map[string]*model.Epic
	provider string
	memo     map[string]bool
	visiting map[string]bool
}

func newProviderReach(byID map[string]*model.Epic, provider string) *providerReach {
	return &providerReach{byID: byID, provider: provider, memo: map[string]bool{}, visiting: map[string]bool{}}
}

// dependsOn reports whether epic id depends on the provider.
func (r *providerReach) dependsOn(id string) bool {
	if got, ok := r.memo[id]; ok {
		return got
	}
	e, ok := r.byID[id]
	if !ok || r.visiting[id] {
		return false
	}
	r.visiting[id] = true
	got := slices.ContainsFunc(e.DependsOn, r.entryReaches)
	delete(r.visiting, id)
	r.memo[id] = got
	return got
}

// isOrDependsOn reports whether id is the provider or depends on it.
func (r *providerReach) isOrDependsOn(id string) bool {
	return id == r.provider || r.dependsOn(id)
}

// entryReaches reports whether one depends_on entry leads to the provider: a
// plain edge through its target, a group only through every member.
func (r *providerReach) entryReaches(d model.EpicDep) bool {
	if !d.IsGroup() {
		return d.ID != "" && r.isOrDependsOn(d.ID)
	}
	members := d.Members()
	return len(members) > 0 && !slices.ContainsFunc(members, func(m string) bool { return !r.isOrDependsOn(m) })
}

// lintEpicTagChecks reports each epic carrying a checked tag that does not
// depend on its provider, or, for a direct check, has no plain direct edge
// to it (spec/1.7.0.md §3). The provider is exempt, and a provider naming no
// epic is dangling-reference's finding.
func lintEpicTagChecks(g *model.EpicGraph) []EpicLintFinding {
	if len(g.TagChecks) == 0 {
		return nil
	}
	byID := epicsByID(g)
	reach := map[string]*providerReach{}
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, tag := range e.Tags {
			check, ok := g.TagChecks[tag]
			if _, exists := byID[check.ProvidedBy]; !ok || !exists || check.ProvidedBy == e.ID {
				continue
			}
			if reach[check.ProvidedBy] == nil {
				reach[check.ProvidedBy] = newProviderReach(byID, check.ProvidedBy)
			}
			if msg := unprovidedTagMessage(e, tag, check, reach[check.ProvidedBy]); msg != "" {
				out = append(out, epicNamedError(EpicRuleUnprovidedTagCheck, msg, []string{tag}, e.ID))
			}
		}
	}
	return out
}

// unprovidedTagMessage words e's tag check finding, or returns "" when the
// check is provided: a direct check by a plain direct edge, any other by e
// depending on the provider as reach judges it.
func unprovidedTagMessage(e *model.Epic, tag string, check model.EpicTagCheck, reach *providerReach) string {
	switch {
	case check.Direct && !dependsDirectlyOn(e, check.ProvidedBy):
		return fmt.Sprintf("epic %q carries tag %q, whose check is direct, but has no direct edge to its provider %q", e.ID, tag, check.ProvidedBy)
	case !check.Direct && !reach.dependsOn(e.ID):
		return fmt.Sprintf("epic %q carries tag %q but does not depend on its provider %q", e.ID, tag, check.ProvidedBy)
	}
	return ""
}

// dependsDirectlyOn reports whether e has a plain edge to provider: the only
// edge a direct tag check accepts. A path through other epics never counts,
// and neither does an any-of group, even one holding the provider.
func dependsDirectlyOn(e *model.Epic, provider string) bool {
	return slices.ContainsFunc(e.DependsOn, func(d model.EpicDep) bool { return !d.IsGroup() && d.ID == provider })
}

// lintEpicNoDependencies warns on each epic outside the first milestone of
// its track that has no dependencies, unless it is marked root or is a
// condition (spec/1.6.0.md F4). An unknown milestone is unknown-milestone's
// finding.
func lintEpicNoDependencies(g *model.EpicGraph) []EpicLintFinding {
	pos := milestonePositions(g)
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		if p, ok := pos[e.Milestone]; ok && p.index > 0 && len(e.DependsOn) == 0 && !e.Root && !e.Condition {
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
	for i := range g.Epics {
		e := &g.Epics[i]
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
// dangling-reference's. An any-of group has no one target, so it is skipped.
func lintEpicThinWhy(g *model.EpicGraph) []EpicLintFinding {
	byID := epicsByID(g)
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			if d.IsGroup() {
				continue
			}
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

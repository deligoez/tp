package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// lintEpicOpensAfter reports each opens_after value that is null or not a
// list (LoadEpicGraphForLint decodes both as null) and each name, key or
// value, that no track lists, by sorted key, then each set of milestones
// opening after each other (spec/1.6.0.md §1). Cycles are read over the
// effective opens_after, so an entry closing a loop with a track's default
// order is one too.
func lintEpicOpensAfter(g *model.EpicGraph) []EpicLintFinding {
	pos := milestonePositions(g)
	var out []EpicLintFinding
	for _, key := range sortedMapKeys(g.OpensAfter) {
		if g.OpensAfter[key] == nil {
			out = append(out, epicNamedError(EpicRuleOpensAfterNotList,
				fmt.Sprintf("opens_after %q is null or not a list: write [] to open it after nothing, or drop the key to open it after the previous milestone of its track", key), []string{key}))
		}
		if _, ok := pos[key]; !ok {
			out = append(out, epicNamedError(EpicRuleUnknownMilestone,
				fmt.Sprintf("opens_after names milestone %q, which is in no track", key), []string{key}))
		}
		for _, name := range g.OpensAfter[key] {
			if _, ok := pos[name]; !ok {
				out = append(out, epicNamedError(EpicRuleUnknownMilestone,
					fmt.Sprintf("opens_after %q opens after milestone %q, which is in no track", key, name), []string{name}))
			}
		}
	}
	return append(out, lintOpensAfterCycles(g, pos)...)
}

// lintOpensAfterCycles reports each strongly connected set of milestones
// over the effective opens_after edges between tracked milestones.
func lintOpensAfterCycles(g *model.EpicGraph, pos map[string]milestonePos) []EpicLintFinding {
	after := EffectiveOpensAfter(g)
	adj := map[string][]string{}
	var nodes []string
	for _, track := range g.Milestones {
		for _, m := range track {
			if _, seen := adj[m]; seen {
				continue
			}
			nodes = append(nodes, m)
			adj[m] = []string{}
			for _, x := range after[m] {
				if _, ok := pos[x]; ok {
					adj[m] = append(adj[m], x)
				}
			}
		}
	}
	var out []EpicLintFinding
	for _, comp := range stronglyConnected(nodes, adj) {
		if len(comp) > 1 || selfLoop(comp[0], adj) {
			names := append([]string(nil), comp...)
			sort.Strings(names)
			out = append(out, epicNamedError(EpicRuleOpensAfterCycle,
				"opens_after cycle among milestones "+strings.Join(names, ", "), names))
		}
	}
	return out
}

// lintEpicPartialParallel reports once per milestone and earlier milestone of
// its track it does not open after, even transitively, when some of its epics
// depend directly on that earlier milestone's epics: the epic block still
// makes them wait (spec/1.6.0.md §1). The finding lists those epics. It is
// info when the milestone's opens_after entry is written without naming the
// earlier one, since the operator chose it, and a warning when the
// parallelism comes from defaults.
func lintEpicPartialParallel(g *model.EpicGraph) []EpicLintFinding {
	after := EffectiveOpensAfter(g)
	byID := epicMilestones(g)
	seen := map[string]bool{}
	var out []EpicLintFinding
	for _, track := range g.Milestones {
		for i, m := range track {
			if seen[m] {
				continue
			}
			seen[m] = true
			waits := opensAfterClosure(after, m)
			for _, earlier := range track[:i] {
				if earlier == m || waits[earlier] {
					continue
				}
				if ids := epicsDependingInto(g, m, earlier, byID); len(ids) > 0 {
					out = append(out, partialParallelFinding(g, m, earlier, ids))
				}
			}
		}
	}
	return out
}

// partialParallelFinding is the partial-parallel finding on milestone m and
// the earlier milestone it does not open after, naming ids: info when m's
// opens_after entry is written, a warning otherwise.
func partialParallelFinding(g *model.EpicGraph, m, earlier string, ids []string) EpicLintFinding {
	verb, them := "depend", "them"
	if len(ids) == 1 {
		verb, them = "depends", "it"
	}
	f := epicWarning(EpicRulePartialParallel, fmt.Sprintf(
		"milestone %q does not open after the earlier %q, yet %s %s on its epics; the epic block still makes %s wait",
		m, earlier, strings.Join(ids, ", "), verb, them), ids...)
	if _, chosen := g.OpensAfter[m]; chosen {
		f.Severity = EpicSeverityInfo
	}
	f.Names = []string{m, earlier}
	sort.Strings(f.Names)
	return f
}

// opensAfterClosure returns every milestone m opens after, transitively.
func opensAfterClosure(after map[string][]string, m string) map[string]bool {
	waits := map[string]bool{}
	stack := append([]string(nil), after[m]...)
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !waits[x] {
			waits[x] = true
			stack = append(stack, after[x]...)
		}
	}
	return waits
}

// epicsDependingInto lists, in graph order and once each, the epics of
// milestone m with a direct dependency on an epic of milestone target.
func epicsDependingInto(g *model.EpicGraph, m, target string, byID map[string]string) []string {
	var ids []string
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.Milestone != m {
			continue
		}
		for _, d := range e.DependsOn {
			if dm, ok := byID[d.ID]; ok && dm == target {
				ids = append(ids, e.ID)
				break
			}
		}
	}
	return ids
}

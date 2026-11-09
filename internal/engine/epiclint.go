package engine

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// Severities of an epic lint finding. An info finding reports a choice the
// graph states outright: it is neither an error nor a warning.
const (
	EpicSeverityError   = "error"
	EpicSeverityWarning = "warning"
	EpicSeverityInfo    = "info"
)

// Structure rules of the epic graph lint (spec/1.3.0.md §4).
const (
	EpicRuleCycle             = "cycle"
	EpicRuleDanglingReference = "dangling-reference"
	EpicRuleDuplicateName     = "duplicate-name"
	EpicRuleUnknownKey        = "unknown-key"
)

// Edge and milestone rules of the epic graph lint (spec/1.3.0.md §4).
const (
	EpicRuleMissingWhy                 = "missing-why"
	EpicRuleUnknownMilestone           = "unknown-milestone"
	EpicRuleLaterMilestoneDependency   = "later-milestone-dependency"
	EpicRuleGateMemberOutsideMilestone = "gate-member-outside-milestone"
	// EpicRuleGateMemberDeferrable is the error on an epic a gate lists both
	// as a member and as deferrable: deferrable epics never hold a gate.
	EpicRuleGateMemberDeferrable = "gate-member-deferrable"
)

// Tag and warning rules of the epic graph lint (spec/1.3.0.md §4).
const (
	EpicRuleUnprovidedTagCheck = "unprovided-tag-check"
	EpicRuleNoDependencies     = "no-dependencies"
	EpicRuleThinWhy            = "thin-why"
	// EpicRuleRootWithDependencies is spec/1.4.0.md §4's warning on an epic
	// marked root that has dependencies.
	EpicRuleRootWithDependencies = "root-with-dependencies"
)

// opens_after rules of the epic graph lint (spec/1.6.0.md §1). A name
// opens_after holds that no track lists is unknown-milestone's finding.
const (
	// EpicRuleOpensAfterCycle is the error on milestones that open after
	// each other, a milestone opening after itself included.
	EpicRuleOpensAfterCycle = "opens-after-cycle"
	// EpicRuleOpensAfterNotList is the error on an opens_after value that is
	// null or not a list, which only a hand edit writes: set and import refuse
	// it, and a null reads as opening after nothing while it looks absent.
	EpicRuleOpensAfterNotList = "opens-after-not-list"
	// EpicRulePartialParallel is the warning, or the info when the
	// milestone's opens_after entry chose it, on a milestone that does not
	// open after an earlier milestone of its track while some of its epics
	// depend directly on that milestone's epics.
	EpicRulePartialParallel = "partial-parallel"
)

// EpicLintFinding is one lint result. Epics holds the sorted, distinct ids the
// finding names, a dangling id included; it is never nil, so it encodes as [].
// Names holds the sorted milestone, gate, tag or key names the finding
// involves: part of its identity across a write (NewEpicErrors), not of the
// payload, where the message carries them. Gate is the milestone whose gate
// list (members or deferrable) holds the id a dangling-reference finding
// names, "" for any other finding: it tells a gate list apart from an edge.
// Origin tells the other dangling-reference shapes that are not edges apart:
// DanglingGateKey or DanglingTagCheck, "" otherwise.
type EpicLintFinding struct {
	Rule     string         `json:"rule"`
	Severity string         `json:"severity"`
	Epics    []string       `json:"epics"`
	Message  string         `json:"message"`
	Names    []string       `json:"-"`
	Gate     string         `json:"-"`
	Origin   DanglingOrigin `json:"-"`
}

// DanglingOrigin tells apart the dangling-reference shapes that name no
// edge; a byte, so a finding stays small enough to range over by value.
type DanglingOrigin uint8

// Origins of a dangling-reference finding.
const (
	// DanglingEdge is a depends_on edge, or any other finding: the zero value.
	DanglingEdge DanglingOrigin = iota
	// DanglingGateKey is a gates key that names no milestone.
	DanglingGateKey
	// DanglingTagCheck is a tag_checks provided_by that names no epic.
	DanglingTagCheck
)

// LintEpicGraph runs the structure, edge, milestone and tag rules, then the
// warnings, over g and the
// unknown keys LoadEpicGraph returned. A cycle finding is one strongly
// connected component; every other rule yields one finding per offending item.
func LintEpicGraph(g *model.EpicGraph, unknown []EpicGraphUnknownKey) []EpicLintFinding {
	var out []EpicLintFinding
	out = append(out, lintEpicCycles(g)...)
	out = append(out, lintEpicDangling(g)...)
	out = append(out, lintEpicDuplicates(g)...)
	out = append(out, lintEpicUnknownKeys(unknown)...)
	out = append(out, lintEpicMissingWhy(g)...)
	out = append(out, lintEpicMilestones(g)...)
	out = append(out, lintEpicOpensAfter(g)...)
	out = append(out, lintEpicGateMembers(g)...)
	out = append(out, lintEpicTagChecks(g)...)
	out = append(out, lintEpicNoDependencies(g)...)
	out = append(out, lintEpicThinWhy(g)...)
	out = append(out, lintEpicRootWithDependencies(g)...)
	out = append(out, lintEpicPartialParallel(g)...)
	return out
}

func epicError(rule, message string, epics ...string) EpicLintFinding {
	ids := []string{}
	seen := map[string]bool{}
	for _, id := range epics {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return EpicLintFinding{Rule: rule, Severity: EpicSeverityError, Epics: ids, Message: message}
}

// epicNamedError is epicError for a finding that involves milestone, gate,
// tag or key names.
func epicNamedError(rule, message string, names []string, epics ...string) EpicLintFinding {
	f := epicError(rule, message, epics...)
	f.Names = append([]string(nil), names...)
	sort.Strings(f.Names)
	return f
}

// epicIDs returns the set of ids the graph defines.
func epicIDs(g *model.EpicGraph) map[string]bool {
	ids := make(map[string]bool, len(g.Epics))
	for i := range g.Epics {
		e := &g.Epics[i]
		ids[e.ID] = true
	}
	return ids
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// lintEpicCycles reports each strongly connected component that holds a
// cycle: two or more epics, or one epic depending on itself.
func lintEpicCycles(g *model.EpicGraph) []EpicLintFinding {
	ids := epicIDs(g)
	adj := map[string][]string{}
	var nodes []string
	for i := range g.Epics {
		e := &g.Epics[i]
		if _, ok := adj[e.ID]; !ok {
			nodes = append(nodes, e.ID)
			adj[e.ID] = nil
		}
		for _, d := range e.DependsOn {
			if ids[d.ID] {
				adj[e.ID] = append(adj[e.ID], d.ID)
			}
		}
	}
	var out []EpicLintFinding
	for _, comp := range stronglyConnected(nodes, adj) {
		if len(comp) > 1 || selfLoop(comp[0], adj) {
			f := epicError(EpicRuleCycle, "", comp...)
			f.Message = "dependency cycle among " + strings.Join(f.Epics, ", ")
			out = append(out, f)
		}
	}
	return out
}

func selfLoop(id string, adj map[string][]string) bool {
	return slices.Contains(adj[id], id)
}

// tarjan holds the state of Tarjan's strongly connected components walk.
type tarjan struct {
	adj     map[string][]string
	index   map[string]int
	low     map[string]int
	onStack map[string]bool
	stack   []string
	next    int
	comps   [][]string
}

// stronglyConnected returns the components of the graph, visiting nodes in
// the given order so the result is deterministic.
func stronglyConnected(nodes []string, adj map[string][]string) [][]string {
	t := &tarjan{adj: adj, index: map[string]int{}, low: map[string]int{}, onStack: map[string]bool{}}
	for _, n := range nodes {
		if _, seen := t.index[n]; !seen {
			t.visit(n)
		}
	}
	return t.comps
}

func (t *tarjan) visit(n string) {
	t.index[n], t.low[n] = t.next, t.next
	t.next++
	t.stack = append(t.stack, n)
	t.onStack[n] = true
	for _, m := range t.adj[n] {
		if _, seen := t.index[m]; !seen {
			t.visit(m)
			t.low[n] = min(t.low[n], t.low[m])
		} else if t.onStack[m] {
			t.low[n] = min(t.low[n], t.index[m])
		}
	}
	if t.low[n] != t.index[n] {
		return
	}
	var comp []string
	for {
		top := t.stack[len(t.stack)-1]
		t.stack = t.stack[:len(t.stack)-1]
		t.onStack[top] = false
		comp = append(comp, top)
		if top == n {
			break
		}
	}
	t.comps = append(t.comps, comp)
}

// lintEpicDangling reports each reference that names no epic, and each gates
// key that names no milestone.
func lintEpicDangling(g *model.EpicGraph) []EpicLintFinding {
	ids := epicIDs(g)
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			if !ids[d.ID] {
				out = append(out, epicError(EpicRuleDanglingReference,
					fmt.Sprintf("epic %q depends on %q, which names no epic", e.ID, d.ID), e.ID, d.ID))
			}
		}
	}
	milestones := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			milestones[m] = true
		}
	}
	for _, name := range sortedMapKeys(g.Gates) {
		out = append(out, lintGateDangling(name, g.Gates[name], ids, milestones)...)
	}
	for _, tag := range sortedMapKeys(g.TagChecks) {
		if p := g.TagChecks[tag].ProvidedBy; !ids[p] {
			f := epicNamedError(EpicRuleDanglingReference,
				fmt.Sprintf("tag_checks %q provided_by %q names no epic", tag, p), []string{tag}, p)
			f.Origin = DanglingTagCheck
			out = append(out, f)
		}
	}
	return out
}

func lintGateDangling(name string, gate model.EpicGate, ids, milestones map[string]bool) []EpicLintFinding {
	var out []EpicLintFinding
	if !milestones[name] {
		f := epicNamedError(EpicRuleDanglingReference,
			fmt.Sprintf("gates key %q names no milestone", name), []string{name})
		f.Origin = DanglingGateKey
		out = append(out, f)
	}
	for _, list := range []struct {
		field string
		ids   []string
	}{{"members", gate.Members}, {"deferrable", gate.Deferrable}} {
		for _, id := range list.ids {
			if !ids[id] {
				f := epicNamedError(EpicRuleDanglingReference,
					fmt.Sprintf("gate %q %s entry %q names no epic", name, list.field, id), []string{name}, id)
				f.Gate = name
				out = append(out, f)
			}
		}
	}
	return out
}

// lintEpicDuplicates reports each epic id, then each milestone name, that
// appears more than once: one finding per duplicated name.
func lintEpicDuplicates(g *model.EpicGraph) []EpicLintFinding {
	var out []EpicLintFinding
	var epicIDList []string
	for i := range g.Epics {
		e := &g.Epics[i]
		epicIDList = append(epicIDList, e.ID)
	}
	for _, id := range repeated(epicIDList) {
		out = append(out, epicError(EpicRuleDuplicateName, fmt.Sprintf("epic id %q appears more than once", id), id))
	}
	var names []string
	for _, track := range g.Milestones {
		names = append(names, track...)
	}
	for _, name := range repeated(names) {
		out = append(out, epicNamedError(EpicRuleDuplicateName,
			fmt.Sprintf("milestone %q appears more than once", name), []string{name}))
	}
	return out
}

// repeated returns the values that occur more than once, in first-seen order.
func repeated(values []string) []string {
	count := map[string]int{}
	var out []string
	for _, v := range values {
		count[v]++
		if count[v] == 2 {
			out = append(out, v)
		}
	}
	return out
}

func lintEpicUnknownKeys(unknown []EpicGraphUnknownKey) []EpicLintFinding {
	out := make([]EpicLintFinding, 0, len(unknown))
	for _, k := range unknown {
		msg := fmt.Sprintf("key %q is outside the schema", k.Path)
		if k.EpicID == "" {
			out = append(out, epicNamedError(EpicRuleUnknownKey, msg, []string{k.Path}))
			continue
		}
		out = append(out, epicNamedError(EpicRuleUnknownKey, msg, []string{k.Path}, k.EpicID))
	}
	return out
}

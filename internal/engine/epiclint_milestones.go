package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// milestonePos is a milestone's place: its track and its index in that track.
type milestonePos struct {
	track, index int
}

// milestonePositions maps each milestone name to its first place in the
// tracks; a repeated name is duplicate-name's finding, not a second place.
func milestonePositions(g *model.EpicGraph) map[string]milestonePos {
	pos := map[string]milestonePos{}
	for t, track := range g.Milestones {
		for i, m := range track {
			if _, ok := pos[m]; !ok {
				pos[m] = milestonePos{track: t, index: i}
			}
		}
	}
	return pos
}

// epicMilestones maps each epic id to the milestone of its first definition;
// a repeated id is duplicate-name's finding.
func epicMilestones(g *model.EpicGraph) map[string]string {
	byID := make(map[string]string, len(g.Epics))
	for _, e := range g.Epics {
		if _, ok := byID[e.ID]; !ok {
			byID[e.ID] = e.Milestone
		}
	}
	return byID
}

// lintEpicMissingWhy reports each edge whose why is absent or blank once
// trimmed, whether or not its target exists.
func lintEpicMissingWhy(g *model.EpicGraph) []EpicLintFinding {
	var out []EpicLintFinding
	for _, e := range g.Epics {
		for _, d := range e.DependsOn {
			if strings.TrimSpace(d.Why) == "" {
				out = append(out, epicError(EpicRuleMissingWhy,
					fmt.Sprintf("edge %q -> %q has no why", e.ID, d.ID), e.ID, d.ID))
			}
		}
	}
	return out
}

// lintEpicMilestones reports each epic whose milestone is absent or in no
// track, then each edge to an epic in a later milestone of the same track.
func lintEpicMilestones(g *model.EpicGraph) []EpicLintFinding {
	pos := milestonePositions(g)
	var out []EpicLintFinding
	for _, e := range g.Epics {
		if _, ok := pos[e.Milestone]; !ok {
			out = append(out, epicNamedError(EpicRuleUnknownMilestone,
				fmt.Sprintf("epic %q has milestone %q, which is in no track", e.ID, e.Milestone), []string{e.Milestone}, e.ID))
		}
	}
	byID := epicMilestones(g)
	for i := range g.Epics {
		out = append(out, lintLaterMilestoneEdges(&g.Epics[i], pos, byID)...)
	}
	return out
}

// lintLaterMilestoneEdges reports e's edges to an epic in a later milestone
// of e's own track. Edges across tracks are free, and an edge an unknown
// milestone or a dangling target leaves unordered is not compared.
func lintLaterMilestoneEdges(e *model.Epic, pos map[string]milestonePos, byID map[string]string) []EpicLintFinding {
	from, ok := pos[e.Milestone]
	if !ok {
		return nil
	}
	var out []EpicLintFinding
	for _, d := range e.DependsOn {
		m, known := byID[d.ID]
		to, placed := pos[m]
		if known && placed && to.track == from.track && to.index > from.index {
			out = append(out, epicNamedError(EpicRuleLaterMilestoneDependency,
				fmt.Sprintf("epic %q in %q depends on %q in the later milestone %q", e.ID, e.Milestone, d.ID, m),
				[]string{e.Milestone, m}, e.ID, d.ID))
		}
	}
	return out
}

// lintEpicGateMembers reports each gate member whose epic is in another
// milestone than the gate's. A gate is its milestone's exit list, so when
// every member sits in one other milestone the gate was keyed by the
// milestone it opens and the message names the key it belongs under. A gates
// key naming no milestone and a member naming no epic are
// dangling-reference's findings, not this rule's.
func lintEpicGateMembers(g *model.EpicGraph) []EpicLintFinding {
	pos := milestonePositions(g)
	byID := epicMilestones(g)
	var out []EpicLintFinding
	for _, name := range sortedMapKeys(g.Gates) {
		if _, ok := pos[name]; !ok {
			continue
		}
		hint := gateKeyHint(name, g.Gates[name].Members, byID)
		for _, id := range g.Gates[name].Members {
			if m, ok := byID[id]; ok && m != name {
				out = append(out, epicNamedError(EpicRuleGateMemberOutsideMilestone,
					fmt.Sprintf("gate %q member %q is in milestone %q%s", name, id, m, hint), []string{name}, id))
			}
		}
	}
	return out
}

// gateKeyHint names the key a gate belongs under when every member it names
// that exists sits in one milestone other than the key's; otherwise "".
func gateKeyHint(name string, members []string, byID map[string]string) string {
	other := ""
	for _, id := range members {
		m, ok := byID[id]
		if !ok {
			continue
		}
		if m == name || (other != "" && m != other) {
			return ""
		}
		other = m
	}
	if other == "" {
		return ""
	}
	return fmt.Sprintf("; a gate is keyed by the milestone its members are in (did you mean gates.%q?)", other)
}

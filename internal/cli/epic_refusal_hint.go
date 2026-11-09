package cli

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

// epicRuleRefusedHints answers a write refused for adding an error of the
// rule: what to change so the write adds none.
var epicRuleRefusedHints = map[string]string{
	engine.EpicRuleCycle:                      "drop or redirect one depends_on edge of the cycle the message names",
	engine.EpicRuleDanglingReference:          "depend only on epics the graph holds (tp epic add adds one), or drop the edge",
	engine.EpicRuleDuplicateName:              "give each epic id and each milestone name one place: rename or remove the duplicate",
	engine.EpicRuleUnknownKey:                 "drop the key the message names: the graph schema does not hold it",
	engine.EpicRuleMissingWhy:                 "give the edge a non-empty why naming what its target supplies",
	engine.EpicRuleUnknownMilestone:           "name a milestone a track lists, or add it to a track first (tp epic milestone add)",
	engine.EpicRuleLaterMilestoneDependency:   "move the edge's target to this or an earlier milestone of the track, or move this epic to a later milestone",
	engine.EpicRuleGateMemberOutsideMilestone: "list as a gate's members or deferrable only epics of its milestone, or move the epic into that milestone",
	engine.EpicRuleGateMemberDeferrable:       "list the epic in the gate's members or its deferrable, not both (tp epic set gates.<m>.members= or .deferrable=)",
	engine.EpicRuleUnprovidedTagCheck:         "make the epic depend on the tag's provider, or drop the tag from the epic",
	engine.EpicRuleOpensAfterCycle:            "drop or redirect one opens_after entry of the cycle; a milestone without an entry opens after the previous one of its track",
}

// epicDanglingOriginHints answer the dangling-reference shapes that are not
// edges, by the finding's Origin.
var epicDanglingOriginHints = map[engine.DanglingOrigin]string{
	engine.DanglingGateKey:  "key a gate by a milestone a track lists (tp epic status lists the milestones), or drop the key",
	engine.DanglingTagCheck: "point provided_by at an epic the graph holds (tp epic add adds one), or drop the tag check",
}

// epicFindingRefusedHint is one added error's hint: a gate list's own for an
// id a gate lists that names no epic, a gates key's or a tag check's own,
// else its rule's, else the generic one.
func epicFindingRefusedHint(f *engine.EpicLintFinding) string {
	if f.Rule == engine.EpicRuleDanglingReference && f.Gate != "" {
		return fmt.Sprintf("list only epics of milestone %q that the graph holds", f.Gate)
	}
	if hint, ok := epicDanglingOriginHints[f.Origin]; ok && f.Rule == engine.EpicRuleDanglingReference {
		return hint
	}
	if f.Rule == engine.EpicRuleDependencyOpensLater {
		return dependencyOpensLaterHint(f)
	}
	if hint, ok := epicRuleRefusedHints[f.Rule]; ok {
		return hint
	}
	return epicWriteRefusedHint
}

// epicRefusedHint is the hint of a refused write: each added error's rule
// hint, once per rule in the order the errors come, and the generic hint for
// a rule that has none.
func epicRefusedHint(refused *engine.EpicWriteRefusedError) string {
	var hints []string
	seen := map[string]bool{}
	for i := range refused.New {
		hint := epicFindingRefusedHint(&refused.New[i])
		if !seen[hint] {
			seen[hint] = true
			hints = append(hints, hint)
		}
	}
	if len(hints) == 0 {
		return epicWriteRefusedHint
	}
	return strings.Join(hints, "; ")
}

// dependencyOpensLaterHint answers a gate member reaching an epic of a
// milestone that opens after the gate's: the finding's Gate is the gate's
// milestone, and its other name the later one.
func dependencyOpensLaterHint(f *engine.EpicLintFinding) string {
	later := f.Gate
	for _, name := range f.Names {
		if name != f.Gate {
			later = name
		}
	}
	return fmt.Sprintf("move the epic of %q the message names to a milestone that opens no later than %q, or drop the dependency, or make the member deferrable", later, f.Gate)
}

package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// epicErrorRules are §4's error-severity rules, the classes a tp epic review
// emission checks mechanically, in ascending order.
var epicErrorRules = []string{
	EpicRuleCycle, EpicRuleDanglingReference, EpicRuleDuplicateName,
	EpicRuleGateMemberDeferrable, EpicRuleGateMemberOutsideMilestone, EpicRuleLaterMilestoneDependency,
	EpicRuleMissingWhy, EpicRuleOpensAfterCycle, EpicRuleOpensAfterNotList, EpicRuleUnknownKey, EpicRuleUnknownMilestone,
	EpicRuleUnprovidedTagCheck,
}

// EpicErrorRules returns the error-severity rule slugs in ascending order: the
// mechanized classes of every tp epic review emission (spec/1.3.0.md §5).
func EpicErrorRules() []string {
	return append([]string(nil), epicErrorRules...)
}

// EpicMechanicalCheck is one error rule run over the whole graph file at an
// emission: Passed is false when the rule fired, Findings how often.
type EpicMechanicalCheck struct {
	Class    string `json:"class"`
	Ran      bool   `json:"ran"`
	Passed   bool   `json:"passed"`
	Findings int    `json:"findings"`
}

// EpicMechanicalChecks turns lint findings into one check per error rule, in
// EpicErrorRules order. Warnings are not checks.
func EpicMechanicalChecks(findings []EpicLintFinding) []EpicMechanicalCheck {
	counts := map[string]int{}
	for _, f := range findings {
		if f.Severity == EpicSeverityError {
			counts[f.Rule]++
		}
	}
	out := make([]EpicMechanicalCheck, 0, len(epicErrorRules))
	for _, rule := range epicErrorRules {
		out = append(out, EpicMechanicalCheck{Class: rule, Ran: true, Passed: counts[rule] == 0, Findings: counts[rule]})
	}
	return out
}

// epicRoundFile names a slice's recorded round N in its state directory.
func epicRoundFile(round int) string {
	return fmt.Sprintf("round-%d.ndjson", round)
}

// epicSnapshotFile names the unit hashes snapshotted when round N was emitted.
func epicSnapshotFile(round int) string {
	return fmt.Sprintf("snapshot-round-%d.json", round)
}

// EpicReviewNextRound returns the round an emission over stateDir opens: one
// past the last consecutively recorded round file, 1 when none is recorded.
func EpicReviewNextRound(stateDir string) int {
	round := 1
	for {
		if _, err := os.Stat(filepath.Join(stateDir, epicRoundFile(round))); err != nil {
			return round
		}
		round++
	}
}

// EpicReviewLastRoundPath returns the path of the slice's last recorded round
// file and its number; round 0 and "" when none is recorded.
func EpicReviewLastRoundPath(stateDir string) (path string, round int) {
	round = EpicReviewNextRound(stateDir) - 1
	if round < 1 {
		return "", 0
	}
	return filepath.Join(stateDir, epicRoundFile(round)), round
}

// epicSnapshot is the file an emission writes: each unit's hash, by key.
type epicSnapshot struct {
	Round  int               `json:"round"`
	Units  map[string]string `json:"units"`
	Blocks map[string]string `json:"blocks,omitempty"`
}

// WriteEpicReviewSnapshot records the unit hashes of round's emission in
// stateDir, creating the directory, through an atomic rename so a partial
// snapshot is never left on disk. Each unit's block text is kept beside its
// hash, so a later round can show what changed. The snapshot stays in the
// state directory and is never printed (§5).
func WriteEpicReviewSnapshot(stateDir string, round int, units []EpicReviewUnit) error {
	snap := epicSnapshot{Round: round, Units: make(map[string]string, len(units)), Blocks: make(map[string]string, len(units))}
	for _, u := range units {
		snap.Units[u.Key] = u.Hash
		snap.Blocks[u.Key] = u.Block
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(stateDir, epicSnapshotFile(round)), append(data, '\n'))
}

// EpicUnitKeys returns the slice's unit keys in slice order.
func EpicUnitKeys(units []EpicReviewUnit) []string {
	keys := make([]string, 0, len(units))
	for _, u := range units {
		keys = append(keys, u.Key)
	}
	return keys
}

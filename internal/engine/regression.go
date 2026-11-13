package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegressionCarried is the one answer to "does this review round carry the
// built-in regression prompt". tp review's default panel asks it to decide
// whether to append the prompt, and tp run's oracle asks it (through
// RegressionDue) to decide whether to spawn a regression unit, so a round can
// never emit a prompt the run leaves unanswered or spawn a unit for a prompt
// the round never emits (spec/backlog/run-regression-unit.md).
//
// A round carries it from round 2 on, outside --no-state, when it has something
// to guard: a section changed or removed since the diff baseline, or at least
// one recorded finding resolved fixed.
func RegressionCarried(round int, noState bool, dr *DiffResult, fixed int) bool {
	if noState || round < 2 {
		return false
	}
	return len(dr.Changed) > 0 || len(dr.Removed) > 0 || fixed > 0
}

// RegressionDue is RegressionCarried over the inputs a counted review round
// reads when no --diff-from is passed, which is how every review-role unit's
// brief emits it: the diff against the newest snapshot earlier than the round,
// and the fixed findings of every review round recorded after the newest epoch.
func RegressionDue(specPath string, round int, st *ReviewState) bool {
	if round < 2 {
		return false
	}
	var dr DiffResult
	if _, snap := NewestEarlierSnapshot(specPath, round); snap != "" {
		dr = DiffSections(sectionDiffLines(snap), sectionDiffLines(specPath))
	}
	fixed := 0
	rounds := RoundsSinceEpoch(st, PhaseReview)
	for i := range rounds {
		rows, _ := LoadRoundRows(specPath, &rounds[i])
		for _, row := range rows {
			if ReviewRowFixed(row) {
				fixed++
			}
		}
	}
	return RegressionCarried(round, false, &dr, fixed)
}

// ReviewRowFixed reports whether a recorded review row is resolved fixed — the
// findings the regression prompt lists and guards.
func ReviewRowFixed(row map[string]any) bool {
	resolved, ok := row["resolved"].(map[string]any)
	if !ok {
		return false
	}
	status, _ := resolved["status"].(string)
	return status == "fixed"
}

// NewestEarlierSnapshot returns the newest round snapshot with a round below r,
// or (0, "") when none exists.
func NewestEarlierSnapshot(specPath string, r int) (round int, path string) {
	for k := r - 1; k >= 1; k-- {
		p := filepath.Join(ReviewStateDir(specPath), fmt.Sprintf("snapshot-round-%d.md", k))
		if _, err := os.Stat(p); err == nil {
			return k, p
		}
	}
	return 0, ""
}

// sectionDiffLines reads and frontmatter-blanks a spec file for section
// diffing. An unreadable file is an empty line set, which is what tp review's
// own reader falls back to after naming the failure on stderr; the oracle has
// no stderr of its own to name it on.
func sectionDiffLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return BlankFrontmatterLines(strings.Split(string(data), "\n"))
}

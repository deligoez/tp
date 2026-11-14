package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// auditRolePools takes each emitted code role's pool, its prompt's file_pool,
// for the round's record to grade against (decision 4 of
// spec/backlog/checklist-covers-what-changed.md), and spec-coverage's asked
// checklist items, its pool being their count (spec/backlog/role-presence.md
// §2.1). It reads the prompts before the --role filter, so every sibling unit
// of a round writes the same pools.
func auditRolePools(prompts []auditPrompt) []engine.AuditRolePool {
	pools := make([]engine.AuditRolePool, 0, len(prompts))
	for i := range prompts {
		if prompts[i].Role == roleSpecCoverage {
			items := checklistIDs(&prompts[i], func(it *ChecklistItem) bool {
				// A carried finding stays under the 1.18.0 carried rule, so
				// one row is never held by two rules.
				return it.Type != "finding"
			})
			pools = append(pools, engine.AuditRolePool{Role: roleSpecCoverage, Pool: len(items), Items: items})
			continue
		}
		items := checklistIDs(&prompts[i], func(it *ChecklistItem) bool { return it.Type == "file_check" })
		pools = append(pools, engine.AuditRolePool{Role: prompts[i].Role, Pool: prompts[i].FilePool, Items: items})
	}
	return pools
}

// checklistIDs returns the ids of a prompt's checklist items keep accepts.
func checklistIDs(p *auditPrompt, keep func(*ChecklistItem) bool) []string {
	ids := make([]string, 0, p.ChecklistCount)
	if p.ChecklistItems == nil {
		return ids
	}
	for j := range *p.ChecklistItems {
		if it := &(*p.ChecklistItems)[j]; keep(it) {
			ids = append(ids, it.ItemID)
		}
	}
	return ids
}

// writeAuditRolePools keeps the round's pools until --record. A failed write
// costs the round its "graded N of M" and nothing else, so it is a notice.
func writeAuditRolePools(specPath string, round int, pools []engine.AuditRolePool) {
	if len(pools) == 0 {
		return
	}
	if err := engine.WriteAuditPools(specPath, round, pools); err != nil {
		output.Notice(fmt.Sprintf("warning: cannot keep audit round %d's file pools, so its record will not say what each role graded: %v", round, err))
	}
}

// gradedAtRecord grades the round's rows against the pools its emission kept.
// Only a round recorded over its own emission's snapshot is graded: a
// no-emission record answers for no reading, whatever pools file sits at its
// number. The pools are kept beside the round file (RecordedAuditPoolsPath),
// so a re-record of the round (TP_ROUND) grades its new rows against the same
// asked items; a round recorded before that copy existed keeps previous, what
// its record stored, and a first record with no pools stores nil.
func gradedAtRecord(specPath string, round int, rows []map[string]any, hashScheme string, previous []engine.RoleGraded) []engine.RoleGraded {
	if hashScheme != engine.HashSchemeSnapshot {
		return nil
	}
	pools, found, err := engine.ReadAuditPools(specPath, round)
	if err != nil {
		output.Notice(fmt.Sprintf("warning: cannot read audit round %d's file pools, so its record says nothing of what each role graded: %v", round, err))
		return nil
	}
	if !found {
		return previous
	}
	if keepErr := engine.KeepRecordedAuditPools(specPath, round, pools); keepErr != nil {
		output.Notice(fmt.Sprintf("warning: cannot keep audit round %d's pools beside its round file, so a re-record keeps this grading: %v", round, keepErr))
	}
	return engine.GradeAuditRoles(pools, rows)
}

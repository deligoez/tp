package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// auditRolePools takes each emitted code role's pool, its prompt's file_pool,
// for the round's record to grade against (decision 4 of
// spec/backlog/checklist-covers-what-changed.md). It reads the prompts before
// the --role filter, so every sibling unit of a round writes the same pools.
func auditRolePools(prompts []auditPrompt) []engine.AuditRolePool {
	pools := make([]engine.AuditRolePool, 0, len(prompts))
	for i := range prompts {
		if prompts[i].Role == roleSpecCoverage {
			continue
		}
		items := make([]string, 0, prompts[i].FileCount)
		if prompts[i].ChecklistItems != nil {
			for j := range *prompts[i].ChecklistItems {
				if it := &(*prompts[i].ChecklistItems)[j]; it.Type == "file_check" {
					items = append(items, it.ItemID)
				}
			}
		}
		pools = append(pools, engine.AuditRolePool{Role: prompts[i].Role, Pool: prompts[i].FilePool, Items: items})
	}
	return pools
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
// number. A re-record of a round (TP_ROUND) finds no pools, since the first
// record removed them, and keeps previous, what that record stored; a first
// record with no pools has nothing to grade against and stores nil.
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
	return engine.GradeAuditRoles(pools, rows)
}

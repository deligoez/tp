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
		pools = append(pools, engine.AuditRolePool{Role: prompts[i].Role, Pool: prompts[i].FilePool})
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

// gradedAtRecord grades the round's rows against the pools its emission kept;
// nil when no emission kept any.
func gradedAtRecord(specPath string, round int, rows []map[string]any) []engine.RoleGraded {
	pools, found, err := engine.ReadAuditPools(specPath, round)
	if err != nil {
		output.Notice(fmt.Sprintf("warning: cannot read audit round %d's file pools, so its record says nothing of what each role graded: %v", round, err))
		return nil
	}
	if !found {
		return nil
	}
	return engine.GradeAuditRoles(pools, rows)
}

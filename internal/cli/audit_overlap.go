package cli

import (
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
)

// computeAuditOverlapReport derives the per-role overlap report (§9.3) for the
// audit phase from merged audit rows. It clusters NON-PASS rows by
// (item_id, category) and credits each cluster's distinct contributing roles,
// mirroring the review overlap report's {role, unique, shared, trim_candidate}
// shape. PASS rows are excluded — a passing checklist item carries no trim
// signal. engine.OverlapReport excludes the reserved regression id and blank
// roles defensively, though audit has no regression role.
func computeAuditOverlapReport(rows []map[string]any) []engine.RoleOverlap {
	type clusterKey struct {
		itemID   string
		category string
	}
	clusters := make(map[clusterKey][]string)
	order := make([]clusterKey, 0)
	for _, r := range rows {
		if asString(r["status"]) == "PASS" {
			continue
		}
		k := clusterKey{itemID: asString(r["item_id"]), category: asString(r["category"])}
		if _, ok := clusters[k]; !ok {
			order = append(order, k)
		}
		clusters[k] = append(clusters[k], asString(r["role"]))
	}
	roleSets := make([][]string, 0, len(order))
	for _, k := range order {
		roleSets = append(roleSets, clusters[k])
	}
	return engine.OverlapReport(roleSets)
}

// latestAuditRoundOverlapReport computes the audit overlap report from the
// latest recorded audit round's NDJSON (§9.3). Returns an empty slice when no
// round is recorded or its file cannot be read.
func latestAuditRoundOverlapReport(specPath string, rounds []engine.ReviewRound) []engine.RoleOverlap {
	if len(rounds) == 0 {
		return []engine.RoleOverlap{}
	}
	last := rounds[len(rounds)-1]
	if last.File == "" {
		return []engine.RoleOverlap{}
	}
	rows, err := parseNDJSONFile(filepath.Join(engine.ReviewStateDir(specPath), last.File))
	if err != nil {
		return []engine.RoleOverlap{}
	}
	return computeAuditOverlapReport(rows)
}

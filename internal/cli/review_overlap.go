package cli

import (
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
)

// computeOverlapReport derives the per-role overlap report (§8.5) from a set of
// merged review findings, each carrying its cluster's found_by_roles.
func computeOverlapReport(findings []map[string]any) []engine.RoleOverlap {
	clusters := make([][]string, 0, len(findings))
	for _, f := range findings {
		clusters = append(clusters, stringSliceField(f["found_by_roles"]))
	}
	return engine.OverlapReport(clusters)
}

// stringSliceField coerces a value into a []string, dropping non-string
// elements. It accepts both an in-memory []string (the merge path, where the
// representative already carries a native slice) and a JSON-decoded []any (the
// status/report path, read back from an NDJSON round file). Any other value
// yields a nil slice.
func stringSliceField(v any) []string {
	switch arr := v.(type) {
	case []string:
		return arr
	case []any:
		out := make([]string, 0, len(arr))
		for _, el := range arr {
			if s, ok := el.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// latestRoundOverlapAndAttribution computes both the overlap report and the
// attribution_excludes list (§9.2) from the latest recorded review round's
// merged findings. Returns an empty report and nil excludes when no round is
// recorded or its NDJSON file cannot be read.
func latestRoundOverlapAndAttribution(specPath string, rounds []engine.ReviewRound) (report []engine.RoleOverlap, excludes []string) {
	if len(rounds) == 0 {
		return []engine.RoleOverlap{}, nil
	}
	last := rounds[len(rounds)-1]
	if last.File == "" {
		return []engine.RoleOverlap{}, nil
	}
	findings, err := parseNDJSONFile(filepath.Join(engine.ReviewStateDir(specPath), last.File))
	if err != nil {
		return []engine.RoleOverlap{}, nil
	}
	return overlapReportWithAttribution(findings)
}

// overlapReportWithAttribution computes the per-role overlap report (§8.5) and,
// per §9.2, the attribution_excludes list. found_by_roles deliberately excludes
// the built-in regression role, so a cluster contributed only by regression
// carries no roles and vanishes from the overlap report while still counting in
// merged_count. attribution_excludes lists "regression" exactly when that
// exclusion caused merged_count to exceed the overlap-report finding count —
// i.e. at least one cluster is regression-only (a non-regression representative
// role with empty found_by_roles is a blank-role cluster, not a regression
// exclusion, and does not trigger the flag). excludes is empty otherwise.
func overlapReportWithAttribution(findings []map[string]any) (report []engine.RoleOverlap, excludes []string) {
	report = computeOverlapReport(findings)
	regressionDropped := 0
	for _, f := range findings {
		if len(stringSliceField(f["found_by_roles"])) > 0 {
			continue
		}
		if asString(f["role"]) == engine.RegressionRoleID {
			regressionDropped++
		}
	}
	if regressionDropped > 0 {
		excludes = []string{engine.RegressionRoleID}
	}
	return report, excludes
}

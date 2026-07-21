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

// latestRoundOverlapReport computes the overlap report from the latest recorded
// review round's merged findings (§8.5). Returns an empty slice when no round is
// recorded or its NDJSON file cannot be read.
func latestRoundOverlapReport(specPath string, rounds []engine.ReviewRound) []engine.RoleOverlap {
	if len(rounds) == 0 {
		return []engine.RoleOverlap{}
	}
	last := rounds[len(rounds)-1]
	if last.File == "" {
		return []engine.RoleOverlap{}
	}
	findings, err := parseNDJSONFile(filepath.Join(engine.ReviewStateDir(specPath), last.File))
	if err != nil {
		return []engine.RoleOverlap{}
	}
	return computeOverlapReport(findings)
}

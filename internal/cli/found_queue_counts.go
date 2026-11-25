package cli

import "github.com/deligoez/tp/internal/engine"

// addFoundOpen puts the number of open queue entries beside taskFilePath on a
// converged review or audit payload (spec/1.31.0.md §6), present at zero. It
// is a count for the reader and never part of the verdict: an open entry holds
// only its finders, so convergence reads as it would without it.
func addFoundOpen(result map[string]any, taskFilePath string, converged bool) {
	if converged {
		result["found_open"] = engine.FoundOpenCount(taskFilePath)
	}
}

package cli

import "github.com/deligoez/tp/internal/engine"

// addLoopDone writes the loop verdict onto a --status or --record payload:
// `done`, and `done_by` naming which condition held (null while the loop is
// open). A loop the cap ended also carries what the cap waived — the rows
// dispositioned fixed that no round re-read, and whether the spec changed after
// the last round — because that is exactly what a clean round would have
// verified. All of it is decision-critical and survives --compact.
func addLoopDone(result map[string]any, d engine.LoopDone) {
	result["done"] = d.Done
	if d.By == "" {
		result["done_by"] = nil
	} else {
		result["done_by"] = d.By
	}
	if d.By == engine.DoneByCap {
		result["fixed_at_cap"] = d.FixedAtCap
		result["stale_waived"] = d.StaleWaived
	}
}

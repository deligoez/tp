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
	if d.BlockingFixedAtCap > 0 {
		result["blocking_fixed_at_cap"] = d.BlockingFixedAtCap
	}
	// What holds a review loop open that no trailing round shows: the carried
	// set's blocking rows, unresolved or fixed and unverified. The count is
	// the whole predicate; the listed rows, up to engine.CarriedRowsShown, are
	// only those of rounds before the latest, since the latest round's own rows
	// are that round's to dispose of. Absent when there are none.
	if len(d.CarriedBlocking) > 0 {
		result["carried_blocking"] = len(d.CarriedBlocking)
		result["carried_blocking_rows"] = engine.CarriedRowsJSON(engine.CarriedEarlier(d.CarriedBlocking))
		if n := engine.CarriedFixedUnverified(d.CarriedBlocking); n > 0 {
			result["carried_fixed_unverified"] = n
		}
	}
}

package engine

import (
	"os"
	"strconv"
)

// RecordTargetRound answers the one question both `--record` paths ask before
// they write anything: which round number does this call record, and does it
// rewrite an already-recorded entry rather than append a new one?
//
// envRound is the raw TP_ROUND value; recorded is how many rounds the phase has
// already stored. Inside a run the round number IS the record unit's id
// (§3.1.1), and the driver hands it to the child as TP_ROUND — so TP_ROUND is
// what identifies the round being re-recorded. §6.3 requires recording to be
// idempotent on that key: re-recording a round already recorded rewrites that
// round's entry rather than adding one, so a retry after a partial failure
// converges on the same state instead of inflating the round count.
//
// Every other input appends recorded+1, which is what tp has always done:
//
//   - TP_ROUND absent or empty — an operator recording by hand. A hand
//     `--record` cannot state which round it means, so treating it as a rewrite
//     would silently overwrite a recorded round. Hand recording stays additive
//     by construction rather than by convention.
//   - A value that is not a positive integer: it identifies no recorded entry.
//   - A value above recorded, including recorded+1 — the round this call is
//     about to create. There is nothing there yet to rewrite.
func RecordTargetRound(envRound string, recorded int) (round int, rewrite bool) {
	if n, err := strconv.Atoi(envRound); err == nil && n >= 1 && n <= recorded {
		return n, true
	}
	return recorded + 1, false
}

// RecordRound applies RecordTargetRound to this process's own TP_ROUND, the way
// Unattended reads TP_UNATTENDED.
func RecordRound(recorded int) (round int, rewrite bool) {
	return RecordTargetRound(os.Getenv(EnvRound), recorded)
}

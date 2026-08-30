package engine

// StopReason is §3.4's closed vocabulary. A run stops for exactly one of nine
// reasons, recorded verbatim in the run state, and every non-converged one is a
// report to a human rather than an acceptance: no stop reason records a round,
// marks a phase converged, or closes a task its own unit did not close.
//
// It is a named type rather than a bare string so the reason a run ends with is
// one of the nine by construction. A caller composing its own string cannot
// reach the recorder, and the caps, the signal handler, the escalation check
// and the exit-code mapping all name the same values instead of each spelling
// them out.
type StopReason string

// The nine, in §3.4's cause-table order. That is the order the table documents
// them in, not the order a checkpoint satisfying several of them resolves in —
// precedence is a separate rule over the same set.
const (
	// StopConverged is the oracle reporting the cycle releasable (§4.1). It
	// is the only reason the loop reaches by agreement rather than by
	// exhaustion, and the only one `tp run` exits 0 on.
	StopConverged StopReason = "converged"
	// The three caps. They bound the run; they never conclude it.
	StopCapUnits     StopReason = "cap-units"
	StopCapWallClock StopReason = "cap-wall-clock"
	StopCapBudget    StopReason = "cap-budget"
	// StopEscalation is a unit that wrote an escalation record (§5.2) — a
	// normal, expected outcome asking for a decision only a user can make.
	StopEscalation StopReason = "escalation"
	// StopUnitFailure is a unit that exhausted its attempts: it exited
	// non-zero, or exited 0 with its durable write absent (§3.3.1).
	StopUnitFailure StopReason = "unit-failure"
	// StopNoUnits is the oracle reporting no pending unit on a cycle that is
	// not releasable — a blocked phase, or one awaiting an operator decision.
	StopNoUnits StopReason = "no-units"
	// StopInterrupted is the operator signalling the driver.
	StopInterrupted StopReason = "interrupted"
	// StopDriverError is a failure the driver itself could not recover from —
	// a runner that will not exec, a run directory it cannot write. It is
	// deliberately not charged to the unit as a failed attempt.
	StopDriverError StopReason = "driver-error"
)

// Known reports whether r is one of §3.4's nine.
//
// The empty string is not one of them: it is what capStop returns while a run
// is within every cap, which is the absence of a stop rather than a stop nobody
// named.
func (r StopReason) Known() bool {
	switch r {
	case StopConverged, StopCapUnits, StopCapWallClock, StopCapBudget,
		StopEscalation, StopUnitFailure, StopNoUnits, StopInterrupted, StopDriverError:
		return true
	}
	return false
}

// stopPrecedence is §3.4's precedence order: when one checkpoint satisfies
// several rows, the recorded reason is the first of these. It is deliberately a
// different order from the cause table the constants are declared in — that
// table names the nine, this names which one a checkpoint that satisfied two of
// them at once records.
//
// converged leads because §3.1 checks releasability before caps, and a cycle
// that became releasable is releasable whatever else the same iteration also
// hit: a cap that trips beside it bounded a run that had already finished, and
// §5.2 says even an escalation raised beside it still records converged.
//
// The order lives here, beside the vocabulary, rather than in the loop that
// consults it, so that ranking two reasons is one lookup instead of an if-chain
// whose test order is the real rule. That is what the loop got wrong: the caps
// were checked at the end of the iteration body, so a run that reached one on
// the very iteration that released the cycle recorded the cap.
var stopPrecedence = [...]StopReason{
	StopConverged,
	StopDriverError,
	StopEscalation,
	StopUnitFailure,
	StopInterrupted,
	StopCapBudget,
	StopCapWallClock,
	StopCapUnits,
	StopNoUnits,
}

// stopRank returns r's position in §3.4's precedence order, lower winning.
//
// A reason the order does not name ranks below every one it does, rather than
// above them. An unranked value is either the empty non-stop or a reason nobody
// placed, and letting one outrank a real reason would hide a genuine stop
// behind a bug; ranking it last still returns it when it is the only candidate,
// where RunRecorder.Stop refuses it at the sink.
func stopRank(r StopReason) int {
	for i, ranked := range stopPrecedence {
		if ranked == r {
			return i
		}
	}
	return len(stopPrecedence)
}

// highestPrecedence returns the one reason a checkpoint satisfying several of
// them records (§3.4), or "" when it satisfied none.
//
// Callers hand it every condition they found true instead of testing them in
// order, so the ordering has exactly one home: a tenth reason cannot be added
// to the loop without being placed in stopPrecedence first, because an unplaced
// one loses to every reason that is.
func highestPrecedence(reasons ...StopReason) StopReason {
	best, bestRank := StopReason(""), len(stopPrecedence)+1
	for _, reason := range reasons {
		if reason == "" {
			continue
		}
		if rank := stopRank(reason); rank < bestRank {
			best, bestRank = reason, rank
		}
	}
	return best
}

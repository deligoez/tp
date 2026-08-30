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

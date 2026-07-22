package engine

// Lifecycle phases reported by the resume oracle (§4.2).
const (
	PhaseReview    = "review"
	PhaseDecompose = "decompose"
	PhaseImplement = "implement"
	PhaseAudit     = "audit"
	PhaseRelease   = "release"
)

// DetectPhase computes the lifecycle phase from durable state using the
// task-first ordering of §4.3, so review convergence — which reads as false
// whenever the spec is stale — can never pull an in-progress project back to
// review. First match wins:
//
//  1. tasks exist and at least one is not done → implement.
//  2. tasks exist, all done, audit not converged → audit.
//  3. tasks exist, all done, audit converged → release.
//  4. zero tasks, review converged → decompose.
//  5. zero tasks, review not converged → review.
//
// numDone is the count of done tasks among numTasks. reviewConverged and
// auditConverged already fold in "no round recorded yet" (both read false with
// zero rounds) and staleness (a stale spec reads unconverged), so an open task
// yields implement even when the spec is stale — the staleness surfaces as a
// blocker (§4.6), it does not change the phase.
func DetectPhase(numTasks, numDone int, reviewConverged, auditConverged bool) string {
	if numTasks > 0 {
		if numDone < numTasks {
			return PhaseImplement
		}
		if auditConverged {
			return PhaseRelease
		}
		return PhaseAudit
	}
	if reviewConverged {
		return PhaseDecompose
	}
	return PhaseReview
}

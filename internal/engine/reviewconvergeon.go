package engine

// Review-convergence policy names for the review_converge_on workflow field.
// blocking (the built-in default) counts a round clean when no surviving finding
// is of a blocking severity; all counts a round clean only when no finding of any
// severity survives (§3.3, §4.1).
const (
	ReviewConvergeOnBlocking = "blocking"
	ReviewConvergeOnAll      = "all"
)

// ReviewConvergeOnHint names the legal review_converge_on values. The set
// commands surface it as the hint on a rejected literal argument (exit 2), and a
// consuming command surfaces it when a stored winning value fails to resolve
// (exit 1) (§3.3).
const ReviewConvergeOnHint = "must be one of: blocking, all"

// ValidReviewConvergeOn reports whether v is a legal review_converge_on value.
// It is the shared predicate the set commands (validating a literal argument)
// and any consuming command (validating a resolved stored value) call, so a
// single vocabulary governs both the usage-error and validation-error paths.
func ValidReviewConvergeOn(v string) bool {
	return v == ReviewConvergeOnBlocking || v == ReviewConvergeOnAll
}

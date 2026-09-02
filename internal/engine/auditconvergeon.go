package engine

// Audit-convergence policy names for the audit_converge_on workflow field.
// all (the built-in default) counts a round clean only when it holds no
// non-PASS row — the behaviour tp shipped before this field existed; blocking
// counts a round clean when no non-PASS row carries a blocking severity (§2).
//
// The values spell the same two words as review_converge_on's, but the two
// fields grade different vocabularies — audit rows are error|warning|info,
// review findings critical|high|… — so the sets are deliberately separate
// constants rather than one shared pair, and the defaults differ: §2.1 measures
// why this one is all where its twin is blocking.
const (
	AuditConvergeOnAll      = "all"
	AuditConvergeOnBlocking = "blocking"
)

// AuditConvergeOnHint names the legal audit_converge_on values. §2 refuses an
// illegal value "on the same terms and with the same hint as
// review_converge_on", so this is byte-identical to ReviewConvergeOnHint rather
// than reordered to put this field's default first: a hint that differs would
// tell a reader the two vocabularies differ, and they do not. A set command
// surfaces it on a rejected literal argument (exit 2) and a consuming audit
// command on an illegal stored value (exit 1).
const AuditConvergeOnHint = "must be one of: blocking, all"

// ValidAuditConvergeOn reports whether v is a legal audit_converge_on value.
// It is the shared predicate the write sinks (validating a literal argument)
// and the consuming audit sinks (validating a resolved stored value) call, so a
// single vocabulary governs both the usage-error and validation-error paths.
//
// Deliberately not the same function as ValidReviewConvergeOn: the two fields
// grade different vocabularies and their defaults differ, so folding them into
// one predicate would make a future divergence in either set silent.
func ValidAuditConvergeOn(v string) bool {
	return v == AuditConvergeOnAll || v == AuditConvergeOnBlocking
}

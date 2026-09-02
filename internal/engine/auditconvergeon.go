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

// AuditConvergeOnRelaxes reports whether a write moving the resolved
// audit_converge_on from before to after relaxes the audit gate — §3's change
// rule, and the whole of it.
//
// It is deliberately a *change* rule rather than the three narrower rules §3
// names and rejects. Fencing the field, or the value blocking, refuses every
// import a project makes once it has resolved blocking, because tp import
// carries the existing block forward — which deadlocks Workflow A step 6 for
// exactly the opt-in users this release is for. Fencing the transition
// all → blocking is walked around by writing blocking over an unset field, and
// misses the import that names neither literal (§7 row 13b).
//
// Both arguments are *resolved* values, not stored ones. Under §2's precedence
// a task override outranks the project config, so a project write of blocking
// beneath a task override of all leaves after equal to before and passes here.
// That is why the comparison takes two resolved values rather than a field name
// and a literal: a caller cannot supply the layer it wrote to instead.
func AuditConvergeOnRelaxes(before, after string) bool {
	return after == AuditConvergeOnBlocking && before != AuditConvergeOnBlocking
}

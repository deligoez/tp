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

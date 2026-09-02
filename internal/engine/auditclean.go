package engine

// advisoryAuditSeverities is the audit vocabulary's below-blocking set (§2).
// Audit rows are error|warning|info and the blocking severity is error, so
// classification is by exclusion: warning and info are advisory and every other
// value blocks. Stating the advisory half rather than the blocking half is what
// makes the rule fail closed — a set naming error would grade everything it
// does not recognise as advisory, which is §7 row 8's mutant.
//
// The review vocabulary is critical|high|medium|low and shares none of these
// words, so this set is deliberately separate from nonBlockingReviewSeverities
// and this file never consults SeverityRank: that ranker is built for the
// review words and ranks every audit severity as unknown (§2).
var advisoryAuditSeverities = map[string]bool{
	"warning": true,
	"info":    true,
}

// auditSeverityBlocking reports whether a non-PASS audit row's severity blocks
// a clean round under the blocking policy (§2). The row's `severity` blocks
// unless it is a JSON string that is exactly `warning` or exactly `info`: a
// string outside the enum, JSON null, an absent key and a value that is not a
// string all block, because a row tp cannot grade is a row tp must not stop
// counting.
//
// The comparison is byte-for-byte, matching AuditRowIsPass's rule for `status`
// in the same vocabulary: no trimming and no case folding, so `WARNING` and
// ` info ` are strings outside the enum and block. That is also what keeps §4's
// identity true — the rows that block are exactly the rows its `error` and
// `unrecognised` buckets hold.
func auditSeverityBlocking(row map[string]any) bool {
	sev, ok := row["severity"].(string)
	if !ok {
		return true
	}
	return !advisoryAuditSeverities[sev]
}

// AuditRowsClean grades one audit round's rows under audit_converge_on (§2).
// Its subject is the round's non-PASS rows: under `all` the round is clean when
// it holds none, and under `blocking` when none of them carries a blocking
// severity. PASS rows are never graded — every recorded PASS row in tp's corpus
// carries `severity: null`, so a predicate phrased over the rows of any
// severity would read that null under the fail-closed rule above and report
// every all-PASS round unclean.
//
// The relaxation is reached only by the exact literal `blocking`; any other
// value, including one a sink failed to refuse, is graded under `all`, which is
// both the stricter policy and this field's built-in default.
//
// This is a pure function of the rows and the policy, not of any stored round:
// §2 stamps `clean` at record time rather than recomputing it live, so callers
// hand it the rows they are recording. That is the one place it differs from
// its review twin, ReviewRoundClean, which reads the round's file on every call
// by design.
func AuditRowsClean(rows []map[string]any, convergeOn string) bool {
	for _, row := range rows {
		if AuditRowIsPass(row) {
			continue
		}
		if convergeOn != AuditConvergeOnBlocking || auditSeverityBlocking(row) {
			return false
		}
	}
	return true
}

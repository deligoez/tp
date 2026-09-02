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
	return !advisoryAuditSeverities[AuditSeverityBucket(row)]
}

// AuditSeverityError and AuditSeverityUnrecognised name the two buckets §4's
// by_severity breakdown does not take verbatim from a row: the blocking
// severity of the audit vocabulary, and the one bucket every row tp cannot
// grade lands in. warning and info key on themselves and are the
// advisoryAuditSeverities set above.
const (
	AuditSeverityError        = "error"
	AuditSeverityUnrecognised = "unrecognised"
)

// AuditSeverityBucket names the by_severity bucket a non-PASS audit row falls
// in (§4). error, warning and info key on themselves; everything else — a
// string outside the enum, JSON null, an absent key, and a value that is not a
// string — keys on unrecognised.
//
// It is the SAME derivation auditSeverityBlocking grades on, not a second copy
// of the vocabulary: that predicate is now phrased over this bucket, so the four
// unrecognised shapes are exactly the rows §2 blocks on and §4's identity —
// error + unrecognised equals the round's blocking-row count — holds by
// construction rather than by two lists agreeing. A separate classifier in the
// merge command is the drift §4 exists to prevent, which is why this is
// exported for internal/cli rather than reimplemented there.
//
// `""` is deliberately not a bucket: it is what the naive type assertion
// row["severity"].(string) produces for three of the four unrecognised shapes,
// and folding them under an empty key is the hiding §4 exists to prevent. The
// comparison stays byte-for-byte with no trimming and no case folding, so
// WARNING is outside the enum, lands in unrecognised, and blocks.
func AuditSeverityBucket(row map[string]any) string {
	sev, ok := row["severity"].(string)
	if !ok {
		return AuditSeverityUnrecognised
	}
	if sev != AuditSeverityError && !advisoryAuditSeverities[sev] {
		return AuditSeverityUnrecognised
	}
	return sev
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

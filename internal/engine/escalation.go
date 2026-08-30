package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// The five decisions §5.2 closes `tp escalate --decision` over. Four name a
// user-only operation §5.1 fences off — the ones a unit can reach and must not
// take itself — and `other` carries everything else an operator has to decide.
//
// The set is closed so that a driver reading a run's records can route them
// without parsing prose: a decision outside it is a usage error, not a sixth
// category.
const (
	EscalateSkipGate       = "skip-gate"
	EscalateRaiseReviewCap = "raise-review-cap"
	EscalateRaiseAuditCap  = "raise-audit-cap"
	EscalateImportForce    = "import-force"
	EscalateOther          = "other"
)

// escalationDecisions is the closed set in the order §5.2 writes it, which is
// also the order a hint lists it in.
var escalationDecisions = []string{
	EscalateSkipGate,
	EscalateRaiseReviewCap,
	EscalateRaiseAuditCap,
	EscalateImportForce,
	EscalateOther,
}

// EscalationDecisions returns the five documented decisions. The copy is
// defensive: the slice reaches a hint builder, and a caller that sorted or
// truncated it in place would silently change what every later escalation is
// validated against.
func EscalationDecisions() []string {
	return slices.Clone(escalationDecisions)
}

// IsEscalationDecision reports whether decision is one of the five. The
// comparison is exact — no case folding, no trimming — because the value is a
// record field a driver switches on, and a decision tp had to guess at is one
// the operator did not write.
func IsEscalationDecision(decision string) bool {
	return slices.Contains(escalationDecisions, decision)
}

// Escalation is §5.2's record: what the unit needs decided, which unit needs
// it, the evidence for the decision, and the options it saw.
//
// It is what a unit produces instead of taking a user-only decision itself, and
// the record — not the exit code — is the signal the driver tests, because the
// driver spawns a harness rather than tp and the harness's exit code need not
// carry the inner command's.
//
// Options is never null in the file. It is documented as an array, and a driver
// that had to treat null and [] alike is a driver tp made work for nothing.
type Escalation struct {
	Decision string   `json:"decision"`
	UnitKind UnitKind `json:"unit_kind"`
	UnitID   string   `json:"unit_id"`
	Phase    string   `json:"phase"`
	Evidence string   `json:"evidence"`
	Options  []string `json:"options"`
	At       string   `json:"at"`
}

// EscalationPath returns one unit's escalation record,
// $TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json (§5.2).
//
// It is keyed by the unit's sequence number rather than by its id, so the two
// role siblings of one round — which share a round and differ only by id and
// seq — never write the same path, and an iteration that produced several
// records keeps all of them.
func EscalationPath(runDir, unitSeq string) string {
	return filepath.Join(runDir, unitSeq+"-escalation.json")
}

// WriteEscalation writes e as the unit's escalation record and returns the path
// it wrote.
//
// It normalizes e in place first, and deliberately owns both normalizations
// alone. Options becomes [] when the caller passed none, because the field is
// documented as an array and a driver that had to treat null and [] alike is a
// driver tp made work for nothing; At is stamped when the caller left it empty,
// so every record's clock is tp's. A second owner in the command layer would
// leave both rules untested through the command that is their only caller.
//
// The write is atomic for the reason the run state's is: the driver reads these
// without taking a lock, and a half-written record would fail validation and be
// judged as no record at all.
func WriteEscalation(runDir, unitSeq string, e *Escalation) (string, error) {
	if e.Options == nil {
		e.Options = make([]string, 0)
	}
	if e.At == "" {
		e.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	record := *e

	path := EscalationPath(runDir, unitSeq)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, append(data, '\n')); err != nil {
		return "", err
	}
	return path, nil
}

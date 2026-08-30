package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// Validate reports why e is not §5.2's record, or nil when it is one.
//
// This is the whole of "schema validation" in §5.2's sense, and it is strict
// about every documented field on purpose: the driver stops a run on this
// record whatever the child exited with, so a record accepted loosely — a
// truncated write, a hand-rolled file, a harness echoing something that looks
// like one — would end a run no unit asked to end. A record that fails here is
// not half-believed: the unit goes back to its §3.3 predicate and its exit
// code, exactly as if it had written nothing.
//
// options is required to be present as an array rather than allowed to be
// null, because WriteEscalation is the record's only writer and always emits
// one: a record without it did not come from the documented path.
func (e *Escalation) Validate() error {
	if !IsEscalationDecision(e.Decision) {
		return fmt.Errorf("decision %q is not one of %s", e.Decision, strings.Join(escalationDecisions, ", "))
	}
	if _, ok := ParseUnitKind(string(e.UnitKind)); !ok {
		return fmt.Errorf("unit_kind %q is not one of the documented unit kinds", e.UnitKind)
	}
	for _, field := range []struct{ name, value string }{
		{"unit_id", e.UnitID},
		{"phase", e.Phase},
		{"evidence", e.Evidence},
		{"at", e.At},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is empty", field.name)
		}
	}
	if e.Options == nil {
		return errors.New("options is absent or null, and is documented as an array")
	}
	return nil
}

// ReadEscalation returns the escalation record at path, or an error when there
// is no valid one there.
//
// Absent, unreadable, unparseable and invalid are deliberately one answer to
// the caller: §5.2 gives all four the same consequence — the unit is judged by
// its predicate and its exit code — and a caller that told them apart would
// have to decide which of them stops a run.
func ReadEscalation(path string) (*Escalation, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the driver's own run directory
	if err != nil {
		return nil, err
	}
	var e Escalation
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &e, nil
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

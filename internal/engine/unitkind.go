package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// UnitKind is one of §3.3's eight unit kinds — a unit being the smallest piece
// of work that ends in a durable write. The string values are the wire form:
// they are what TP_UNIT_KIND carries, what next_units reports and what a
// per-kind runner map keys on, so they are the table's spellings byte for byte.
type UnitKind string

const (
	UnitImplement     UnitKind = "implement"
	UnitReviewRole    UnitKind = "review-role"
	UnitReviewRecord  UnitKind = "review-record"
	UnitReviewResolve UnitKind = "review-resolve"
	UnitDecompose     UnitKind = "decompose"
	UnitAuditRole     UnitKind = "audit-role"
	UnitAuditRecord   UnitKind = "audit-record"
	UnitAuditFix      UnitKind = "audit-fix"
)

// unitKindOrder lists the eight kinds in §3.3's table order. It is the single
// source for both the exported set and the validity test, so a kind can never
// be valid without being listed.
var unitKindOrder = []UnitKind{
	UnitImplement,
	UnitReviewRole,
	UnitReviewRecord,
	UnitReviewResolve,
	UnitDecompose,
	UnitAuditRole,
	UnitAuditRecord,
	UnitAuditFix,
}

// UnitKinds returns the eight kinds in §3.3's table order, for callers that
// need to name the set rather than test one value. The copy keeps a caller from
// reordering the sequence every other reader depends on.
func UnitKinds() []UnitKind {
	out := make([]UnitKind, len(unitKindOrder))
	copy(out, unitKindOrder)
	return out
}

// ParseUnitKind maps a wire value — a TP_UNIT_KIND, a runner map key, a
// recorded run-state row — onto a kind, reporting false for anything outside
// the eight. It does not trim or case-fold: the values are tp's own, written by
// tp, so a near-miss is a caller's bug and surfacing it is the point.
func ParseUnitKind(s string) (UnitKind, bool) {
	k := UnitKind(s)
	if slices.Contains(unitKindOrder, k) {
		return k, true
	}
	return "", false
}

// UnitConcurrency is §3.3's concurrency column.
type UnitConcurrency string

const (
	// ConcurrencyAlone is the table's "Alone": the unit runs with no sibling
	// beside it.
	ConcurrencyAlone UnitConcurrency = "alone"
	// ConcurrencySiblingRoles is the table's "Parallel with sibling roles": the
	// unit may run beside other role units of the same phase and round, which
	// is safe because each writes only its own findings file (§3.3.1).
	ConcurrencySiblingRoles UnitConcurrency = "sibling-roles"
)

// Concurrency returns the kind's §3.3 concurrency. Only the two role kinds run
// beside a sibling; every other kind — and any value outside the enum — is
// alone, which is the safe direction: running an unrecognized kind concurrently
// is what would interleave two writers of the same file.
func (k UnitKind) Concurrency() UnitConcurrency {
	if k == UnitReviewRole || k == UnitAuditRole {
		return ConcurrencySiblingRoles
	}
	return ConcurrencyAlone
}

// Concurrent reports whether the kind may be spawned alongside another unit.
// It is the driver's and the oracle's test; Concurrency is the table datum it
// reads.
func (k UnitKind) Concurrent() bool {
	return k.Concurrency() == ConcurrencySiblingRoles
}

// mergedFindingsName is the round's merged findings/results file, written by the
// round's record unit inside TP_ROUND_DIR and read by the resolve and fix kinds.
const mergedFindingsName = "merged.ndjson"

// RoleFindingsPath returns a role unit's findings file for a round:
// $TP_ROUND_DIR/role-<id>.ndjson. The final name, not the .part a role unit
// writes and the driver renames on exit 0 (§3.3.1) — the predicate reads what
// the rename produced.
func RoleFindingsPath(roundDir, roleID string) string {
	return filepath.Join(roundDir, "role-"+roleID+".ndjson")
}

// MergedFindingsPath returns a round's merged findings file,
// $TP_ROUND_DIR/merged.ndjson: what the record unit writes, what
// review-resolve and audit-fix dispose rows in.
func MergedFindingsPath(roundDir string) string {
	return filepath.Join(roundDir, mergedFindingsName)
}

// roundFilePath returns the recorded round file for a phase and round number —
// <spec-dir>/.tp-review/<base>/{review,audit}-round-<N>.ndjson. The names mirror
// the ones tp review --record and tp audit --record write.
func roundFilePath(specPath, phase string, round int) string {
	return filepath.Join(ReviewStateDir(specPath), fmt.Sprintf("%s-round-%d.ndjson", phase, round))
}

// UnitTarget names the artifacts a durable-write predicate reads: the driver's
// resolved task file (TP_FILE), the resolved spec, the round directory
// (TP_ROUND_DIR) and its round number (TP_ROUND), and the unit's own id
// (TP_UNIT_ID). Each kind reads only the fields §3.3 names for it, so a target
// carrying just those is enough.
type UnitTarget struct {
	TaskFile string
	Spec     string
	RoundDir string
	Round    int
	ID       string
}

// DurableWrite reports whether the unit's §3.3 durable write is present.
//
// Every predicate is a state, never a delta: each is decided by reading the
// named artifacts as they are, with no baseline and no "before". That is what
// lets the Stop hook (§6.2) and the driver's own success test read the same
// condition, and it is why a unit that correctly closes a finding without
// changing any code still passes.
//
// An artifact that is absent or unreadable is a durable write that is not
// present, so the predicate answers false rather than an error: that state is
// exactly "the unit did not finish". Telling a crash from an unfinished unit is
// the exit code's job, which §3.3.1 pairs with this one — a unit succeeded when
// it exited 0 AND this is true; either alone is a failed attempt.
func (k UnitKind) DurableWrite(t UnitTarget) bool {
	switch k {
	case UnitImplement:
		return taskIsDone(t.TaskFile, t.ID)
	case UnitReviewRole, UnitAuditRole:
		return roleFindingsParse(RoleFindingsPath(t.RoundDir, t.ID))
	case UnitReviewRecord:
		return fileExists(MergedFindingsPath(t.RoundDir)) && fileExists(roundFilePath(t.Spec, "review", t.Round))
	case UnitAuditRecord:
		return fileExists(MergedFindingsPath(t.RoundDir)) && fileExists(roundFilePath(t.Spec, "audit", t.Round))
	case UnitReviewResolve:
		return allRowsDisposed(MergedFindingsPath(t.RoundDir))
	case UnitDecompose:
		return taskFileHasTasks(t.TaskFile)
	case UnitAuditFix:
		return rowIsDisposed(MergedFindingsPath(t.RoundDir), t.ID)
	default:
		return false
	}
}

// fileExists reports whether path names something that can be stat'd. The
// record kinds' halves are existence checks and nothing more (§3.3.1).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// taskIsDone reports whether the task file at path holds a task with this id
// whose status is done — the implement kind's predicate. Read from disk on
// every call, because the child closed the task in its own process.
func taskIsDone(path, id string) bool {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return false
	}
	task, _, err := model.FindTask(tf, id)
	if err != nil {
		return false
	}
	return task.Status == model.StatusDone
}

// taskFileHasTasks reports whether the task file at path holds at least one
// task — the decompose kind's predicate. The init shell holds zero tasks, so an
// import that wrote nothing is not a completed decompose.
func taskFileHasTasks(path string) bool {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return false
	}
	return len(tf.Tasks) > 0
}

// roleFindingsParse reports whether a role's findings file exists and every one
// of its content lines parses — the role kinds' predicate.
//
// It reads only content lines: blank and whitespace-only lines are ignored,
// matching §8a.4, so a trailing newline never fails a role and a file of
// nothing but blank lines passes (a role with no findings is a clean role, not
// a failed unit). The parse test is the record path's byte for byte — a line
// must unmarshal into a JSON object — so a file tp itself would accept can
// never fail here, including the `null` line Go accepts into a nil map.
func roleFindingsParse(path string) bool {
	_, ok := readNDJSONRows(path)
	return ok
}

// allRowsDisposed reports whether every finding in the round's merged file
// carries a disposition — the review-resolve kind's predicate. A finding is
// disposed when it carries the `resolved` object tp review --resolve writes.
// A merged file that is absent or holds an unparseable line fails: the round's
// findings cannot be shown to be disposed if they cannot be read.
func allRowsDisposed(path string) bool {
	rows, ok := readNDJSONRows(path)
	if !ok {
		return false
	}
	for _, row := range rows {
		if _, disposed := row["resolved"]; !disposed {
			return false
		}
	}
	return true
}

// rowIsDisposed reports whether the row selected by a `role:item_id` key
// carries a disposition in the round's results file — the audit-fix kind's
// predicate. That is the whole predicate: a finding correctly closed as wontfix
// or duplicate, with no code change at all, satisfies it (§3.3.1).
//
// The selector splits on the first colon, since a role id never contains one.
// A key naming no row in the file is false — a unit cannot have disposed a row
// that is not there.
func rowIsDisposed(path, key string) bool {
	role, itemID, found := strings.Cut(key, ":")
	if !found {
		return false
	}
	rows, ok := readNDJSONRows(path)
	if !ok {
		return false
	}
	for _, row := range rows {
		if AuditRowRole(row) != role {
			continue
		}
		if id, _ := row["item_id"].(string); id != itemID {
			continue
		}
		_, disposed := row["resolved"]
		return disposed
	}
	return false
}

// readNDJSONRows reads an NDJSON artifact into its content rows, reporting
// false when the file cannot be read or any content line does not unmarshal
// into a JSON object. Blank and whitespace-only lines are skipped rather than
// counted, which is the blank-line rule §3.3 states for role files and the same
// rule the record path applies to a recorded round file.
func readNDJSONRows(path string) ([]map[string]any, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	rows := make([]map[string]any, 0)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row map[string]any
		if unmarshalErr := json.Unmarshal([]byte(line), &row); unmarshalErr != nil {
			return nil, false
		}
		rows = append(rows, row)
	}
	return rows, true
}

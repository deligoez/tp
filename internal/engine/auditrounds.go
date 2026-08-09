package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/output"
)

// AuditRowRole returns a recorded audit row's role id (§2.1): the `role` field,
// whitespace-trimmed, when that field is a JSON string that is non-empty after
// trimming. An absent key, a non-string value, an empty string and a
// whitespace-only string all mean the row carries no role, reported as "".
//
// The trimmed value is the role id and is compared byte-for-byte by every
// caller, never case-folded: corpus ids are lowercase kebab-case filenames, so
// "Spec-Coverage" is a different id from "spec-coverage" and has to surface as
// one rather than be silently merged into it. The reserved `regression` id is
// not special-cased either — the audit corpus has no such role, so an audit row
// can only carry it by a sub-agent's mistake, and treating it as an ordinary
// role makes that mistake visible instead of dropping it silently.
//
// This matches the trim-then-drop-empty rule OverlapReport applies. It
// deliberately does not match the untrimmed `role != ""` test the roleless-row
// advisory uses, so a row whose role is whitespace-only is unattributed here
// while that advisory still counts it as attributed; that advisory is unchanged.
func AuditRowRole(row map[string]any) string {
	s, ok := row["role"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// AuditRowIsPass reports whether a recorded audit row is PASS (§2.1): its
// `status` field is a JSON string exactly equal to "PASS" — no trimming, no
// case folding. Everything else is non-PASS: an absent key, a non-string value,
// "pass", " PASS ", "FAIL", "PARTIAL".
//
// This is byte-for-byte the predicate the audit record path applies when it
// decides a round's recorded `findings` count, and it must stay identical to
// it: were the two to drift, the per-role open counts and the round's own
// stored `findings` would disagree about the same rows.
func AuditRowIsPass(row map[string]any) bool {
	status, _ := row["status"].(string)
	return status == "PASS"
}

// ReadAuditRoundRows reads one recorded audit round's rows under §2.1's rules.
// latestRolesHash is the stored roles_hash of the latest recorded audit round —
// the panel now in force — against which entry's own stored hash is judged.
//
// It returns (rows, true) when the round contributes rows, and (nil, false)
// after emitting exactly one output.Notice naming the first cause that applied
// when it contributes none. A round contributes no rows when its stored
// roles_hash is empty or differs from the latest round's, when its `file` entry
// is empty, when the recorded NDJSON file cannot be read, or when any non-blank
// line in it fails to unmarshal into a JSON map.
//
// The causes are not mutually exclusive, so the evaluation order is fixed. The
// stored-roles_hash test is decided first, because it needs no file read, and
// within it the empty case is decided before the differing case, since an empty
// hash is not a value that can differ from anything: it means tp does not know
// which panel the round used, and naming that a different corpus would report a
// corpus change that may never have happened. The three file-based causes then
// follow in §2.1's table order.
//
// A line is blank when it is empty after trimming whitespace — the record
// path's own test, so a CRLF-terminated results file behaves the same in both,
// and a recorded file's trailing newline is not a parse failure. The unmarshal
// test is the record path's byte for byte, and its edge is deliberate: Go
// accepts the bare literal `null` into a map and leaves the map nil, so a
// `null` line is a row here exactly as it is a counted finding there — one
// carrying no role and no status. Numbers, strings, booleans and arrays all
// error and take the no-rows path. Matching the record path is what keeps the
// two from disagreeing about a file tp itself accepted, and it buys the
// guarantee that silently dropping a bad line can never delete a role's only
// non-PASS row — which would make a role clean, lengthen a streak, and make the
// divergence signal more likely to fire, the one direction §1 forbids.
//
// §2 reads rounds here rather than through LoadRoundRows, which cannot express
// these rules: that function returns the same "not found" for an unreadable
// file and for an empty `file` entry, and it silently skips a line that does
// not unmarshal. LoadRoundRows and its callers are unchanged.
func ReadAuditRoundRows(specPath string, entry *ReviewRound, latestRolesHash string) (rows []map[string]any, ok bool) {
	if entry.RolesHash == "" {
		output.Notice(fmt.Sprintf("round %d has no auditor-corpus hash; skipping its rows", entry.Round))
		return nil, false
	}
	if entry.RolesHash != latestRolesHash {
		output.Notice(fmt.Sprintf("round %d was recorded under a different auditor corpus; skipping its rows", entry.Round))
		return nil, false
	}
	if entry.File == "" {
		output.Notice(fmt.Sprintf("round %d has no recorded rows file; skipping its rows", entry.Round))
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(ReviewStateDir(specPath), entry.File))
	if err != nil {
		output.Notice(fmt.Sprintf("round %d file %s is missing; skipping its rows", entry.Round, entry.File))
		return nil, false
	}
	rows = make([]map[string]any, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if unmarshalErr := json.Unmarshal([]byte(line), &m); unmarshalErr != nil {
			output.Notice(fmt.Sprintf("round %d file %s has unparseable rows; skipping its rows", entry.Round, entry.File))
			return nil, false
		}
		rows = append(rows, m)
	}
	return rows, true
}

package cli

import (
	"encoding/json"
	"maps"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

// auditCarryKey is the join key of the audit carry: a role's item, the same
// (role, item_id) pair the Prior Round block and --merge key rows by.
type auditCarryKey struct {
	role   string
	itemID string
}

func auditCarryKeyOf(row map[string]any) auditCarryKey {
	role, _ := row["role"].(string)
	itemID, _ := row["item_id"].(string)
	return auditCarryKey{role: role, itemID: itemID}
}

// carryAuditAcceptances is the audit carry at --record: an acceptance the
// operator made in the round immediately before this one is re-applied to the
// same item's new non-PASS row, so an accepted finding the auditor records
// again does not dirty every round after the one it was accepted in.
//
// Only wontfix and duplicate with evidence are carried (engine.RowAccepted);
// fixed never is, because a repair is re-verified by the next round, not
// inherited. A target row that carries a disposition of its own keeps it, and
// a PASS needs none. The carry breaks when the prior row's evidence_file was
// touched by a commit since that round was recorded — the changed_since the
// Prior Round block shows the auditor — and a row with no evidence_file has no
// file to change, so its acceptance carries until the operator re-decides it.
// The lookup is the immediately prior round alone, as ground's carry is, so
// the chain runs round by round and one edit ends it.
//
// The carried disposition is the prior `resolved` block copied verbatim, with
// `carried_from` added when it has none: like ground's, it names the round the
// disposition was first decided in, so a chain keeps pointing at the
// operator's decision. The carry re-applies that decision and writes no new
// acceptance, so the accept-finding fence --resolve applies has no place here.
//
// rows are the parsed non-blank lines of data, in order; a carried row is
// updated in place, so the clean verdict computed from rows reads it, and its
// line of data is re-encoded. It returns data unchanged when nothing carries.
func carryAuditAcceptances(specPath string, recorded []engine.ReviewRound, data []byte, rows []map[string]any) (out []byte, carried int) {
	round, _ := engine.RecordRound(len(recorded))
	if round < 2 || round-2 >= len(recorded) {
		return data, 0
	}
	prev := &recorded[round-2]
	source := acceptedAuditRows(specPath, prev)
	if len(source) == 0 {
		return data, 0
	}
	lines := strings.Split(string(data), "\n")
	next := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || next >= len(rows) {
			continue
		}
		row := rows[next]
		next++
		resolved, ok := carriedDisposition(row, source, round-1)
		if !ok {
			continue
		}
		row["resolved"] = resolved
		// A row decoded from JSON always re-encodes.
		encoded, _ := json.Marshal(row)
		lines[i] = string(encoded)
		carried++
	}
	if carried == 0 {
		return data, 0
	}
	return []byte(strings.Join(lines, "\n")), carried
}

// acceptedAuditRows returns the prior round's accepted non-PASS rows whose
// evidence_file is unchanged since that round was recorded, keyed by
// (role, item_id), each as its `resolved` block. The first row under a key
// decides it, as in ground's carry. Git is asked only when some accepted row
// names a file.
func acceptedAuditRows(specPath string, prev *engine.ReviewRound) map[auditCarryKey]map[string]any {
	rows, found := engine.LoadRoundRows(specPath, prev)
	if !found {
		return nil
	}
	var changed map[string]bool
	seen := make(map[auditCarryKey]bool)
	source := make(map[auditCarryKey]map[string]any)
	for _, row := range rows {
		key := auditCarryKeyOf(row)
		if seen[key] || engine.AuditRowIsPass(row) || !engine.RowAccepted(row) {
			continue
		}
		seen[key] = true
		if ef, _ := row["evidence_file"].(string); ef != "" {
			if changed == nil {
				changed = filesChangedSinceRound(specPath, prev)
			}
			if changed[ef] {
				continue
			}
		}
		source[key], _ = row["resolved"].(map[string]any)
	}
	return source
}

// carriedDisposition returns the disposition row inherits from source: a copy
// of the prior `resolved` block stamped with carried_from, or false when row is
// a PASS, carries a disposition of its own, or has no accepted prior row.
func carriedDisposition(row map[string]any, source map[auditCarryKey]map[string]any, prevRound int) (map[string]any, bool) {
	if engine.AuditRowIsPass(row) {
		return nil, false
	}
	if _, own := row["resolved"]; own {
		return nil, false
	}
	prior, ok := source[auditCarryKeyOf(row)]
	if !ok {
		return nil, false
	}
	resolved := maps.Clone(prior)
	if _, has := resolved["carried_from"]; !has {
		resolved["carried_from"] = prevRound
	}
	return resolved, true
}

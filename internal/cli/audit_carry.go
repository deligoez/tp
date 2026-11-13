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
// operator made on an item's latest graded row is re-applied to the same
// item's new non-PASS row, so an accepted finding the auditor records again
// does not dirty every round after the one it was accepted in.
//
// Only wontfix and duplicate with evidence are carried (engine.RowAccepted);
// fixed never is, because a repair is re-verified by a later round, not
// inherited. A target row that carries a disposition of its own keeps it, and
// a PASS needs none. The carry breaks when the accepted row's evidence_file
// was touched by a commit since its round was recorded — the changed_since
// the Prior Round block shows the auditor — and a row with no evidence_file
// has no file to change, so its acceptance carries until the operator
// re-decides it.
//
// The source is the audit carried set (engine.AuditCarriedSet): per
// (role, item_id) since the newest epoch, the row of the item's latest graded
// round, so an empty round between does not end the chain, while a re-grade
// does, and so does an epoch: a round at or below the cut was recorded about
// a text the spec no longer holds, and the operator is asked again
// (tp review --rewritten). This is the row the Prior Round block lists
// (spec/backlog/audit-carried-fail-blocks.md §2.3).
//
// The carried disposition is the accepted `resolved` block copied verbatim,
// with `carried_from` added when it has none: like ground's, it names the
// round the disposition was first decided in, so a chain keeps pointing at
// the operator's decision. The carry re-applies that decision and writes no
// new acceptance, so the accept-finding fence --resolve applies has no place
// here.
//
// rows are the parsed non-blank lines of data, in order; a carried row is
// updated in place, so the clean verdict computed from rows reads it, and its
// line of data is re-encoded. It returns data unchanged when nothing carries.
func carryAuditAcceptances(specPath string, st *engine.ReviewState, data []byte, rows []map[string]any) (out []byte, carried int) {
	source := acceptedAuditRows(specPath, st)
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
		resolved, ok := carriedDisposition(row, source)
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

// acceptedAuditSource is an accepted row the carry may re-apply: its
// `resolved` block and the round it was graded in.
type acceptedAuditSource struct {
	resolved map[string]any
	round    int
}

// acceptedAuditRows returns the carried set's accepted rows whose
// evidence_file is unchanged since their round was recorded, keyed by
// (role, item_id). Git is asked once per round some accepted row names a file.
func acceptedAuditRows(specPath string, st *engine.ReviewState) map[auditCarryKey]acceptedAuditSource {
	if st == nil {
		return nil
	}
	set, _ := engine.AuditCarriedSet(specPath, st)
	byNumber := make(map[int]*engine.ReviewRound, len(st.AuditRounds))
	for i := range st.AuditRounds {
		byNumber[st.AuditRounds[i].Round] = &st.AuditRounds[i]
	}
	changedByRound := make(map[int]map[string]bool)
	source := make(map[auditCarryKey]acceptedAuditSource)
	for i := range set {
		row := set[i].Row
		if !engine.RowAccepted(row) {
			continue
		}
		if ef, _ := row["evidence_file"].(string); ef != "" {
			changed, ok := changedByRound[set[i].Round]
			if !ok {
				changed = filesChangedSinceRound(specPath, byNumber[set[i].Round])
				changedByRound[set[i].Round] = changed
			}
			if changed[ef] {
				continue
			}
		}
		resolved, _ := row["resolved"].(map[string]any)
		source[auditCarryKeyOf(row)] = acceptedAuditSource{resolved: resolved, round: set[i].Round}
	}
	return source
}

// carriedDisposition returns the disposition row inherits from source: a copy
// of the accepted `resolved` block stamped with carried_from, or false when
// row is a PASS, carries a disposition of its own, or has no accepted source.
func carriedDisposition(row map[string]any, source map[auditCarryKey]acceptedAuditSource) (map[string]any, bool) {
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
	resolved := maps.Clone(prior.resolved)
	if _, has := resolved["carried_from"]; !has {
		resolved["carried_from"] = prior.round
	}
	return resolved, true
}

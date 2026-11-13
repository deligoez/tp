package engine

import (
	"path/filepath"
	"sort"
	"strings"
)

// auditKey is an audit row's identity across rounds: its role and item, the
// (role, item_id) pair the Prior Round block, --merge and the acceptance carry
// key rows by.
type auditKey struct {
	role string
	item string
}

func auditKeyOf(row map[string]any) auditKey {
	return auditKey{role: rowString(row, "role"), item: rowString(row, "item_id")}
}

// positionalAuditID reports whether an audit item id names a position in the
// spec's text rather than a thing: a table row, a list item, or a finding's
// ordinal. After the text changes such an id may name another item, so a PASS
// over a different text does not clear a row under it
// (spec/backlog/audit-carried-fail-blocks.md §2.2). task- and file- ids name a
// task and a path.
func positionalAuditID(id string) bool {
	return strings.HasPrefix(id, "table-") || strings.HasPrefix(id, "list-") || strings.HasPrefix(id, "finding-")
}

// auditSeverityRank orders the audit severity buckets from the least severe:
// info, warning, then error and unrecognised, which both block.
func auditSeverityRank(bucket string) int {
	switch bucket {
	case "info":
		return 0
	case "warning":
		return 1
	}
	return 2
}

// auditRowWeight orders the rows one round holds under one key, the worst
// highest: an unresolved non-PASS row over a fixed one over an accepted one,
// any of them over a PASS, and inside each the more severe.
func auditRowWeight(row map[string]any) int {
	if AuditRowIsPass(row) {
		return 0
	}
	resolved, has := row["resolved"].(map[string]any)
	base := 30
	switch CarryDisposition(has, rowString(resolved, "status"), rowString(resolved, "evidence")) {
	case CarryAccepted:
		base = 10
	case CarryResolved:
		base = 20
	}
	return base + auditSeverityRank(AuditSeverityBucket(row))
}

// auditOpen is a key's carried row while the rounds are read, with the spec
// hash of its round and its severity so far.
type auditOpen struct {
	row  CarriedRow
	hash string
	sev  string
}

// AuditCarriedSet is the audit carried set (spec/backlog/
// audit-carried-fail-blocks.md §2.1): per (role, item_id) since the newest
// epoch, the row of its latest round, the worst of that round's rows under
// the key. A later PASS clears the key, unless the id is positional and the
// PASS graded another text; an acceptance stays in the set as accepted. Each
// row is graded at the highest severity among its rows since it was last
// cleared or accepted. Rows are in round and index order; missing lists the
// rounds whose file could not be read.
func AuditCarriedSet(specPath string, st *ReviewState) (rows []CarriedRow, missing []ReviewRound) {
	rounds := RoundsSinceEpoch(st, PhaseAudit)
	open := make(map[auditKey]*auditOpen)
	for i := range rounds {
		r := &rounds[i]
		recorded, found := LoadRoundRows(specPath, r)
		if !found {
			missing = append(missing, *r)
			continue
		}
		file := filepath.Join(ReviewStateDir(specPath), r.File)
		keys, worst := worstAuditRows(recorded)
		for _, k := range keys {
			j := worst[k]
			carryAuditRow(open, k, r, file, j, recorded[j])
		}
	}
	latest := 0
	if st != nil {
		latest = latestRecordedRound(st.AuditRounds)
	}
	rows = make([]CarriedRow, 0, len(open))
	for _, o := range open {
		row := o.row
		row.Severity = o.sev
		row.Earlier = row.Round < latest
		rows = append(rows, row)
	}
	sort.Slice(rows, func(a, b int) bool {
		if rows[a].Round != rows[b].Round {
			return rows[a].Round < rows[b].Round
		}
		return rows[a].Index < rows[b].Index
	})
	return rows, missing
}

// worstAuditRows returns a round's keys in the order they first appear and,
// per key, the index of its worst row.
func worstAuditRows(recorded []map[string]any) (keys []auditKey, worst map[auditKey]int) {
	worst = make(map[auditKey]int)
	for j, row := range recorded {
		k := auditKeyOf(row)
		prev, seen := worst[k]
		switch {
		case !seen:
			keys = append(keys, k)
			worst[k] = j
		case auditRowWeight(row) > auditRowWeight(recorded[prev]):
			worst[k] = j
		}
	}
	return keys, worst
}

// carryAuditRow applies one round's worst row under k to the open set.
func carryAuditRow(open map[auditKey]*auditOpen, k auditKey, r *ReviewRound, file string, index int, row map[string]any) {
	cur := open[k]
	if AuditRowIsPass(row) {
		if cur != nil && positionalAuditID(k.item) && cur.hash != r.SpecHash {
			return
		}
		delete(open, k)
		return
	}
	resolved, has := row["resolved"].(map[string]any)
	disp := CarryDisposition(has, rowString(resolved, "status"), rowString(resolved, "evidence"))
	sev := AuditSeverityBucket(row)
	if cur != nil && cur.row.Disposition != CarryAccepted && auditSeverityRank(cur.sev) > auditSeverityRank(sev) {
		sev = cur.sev
	}
	open[k] = &auditOpen{
		row:  CarriedRow{Round: r.Round, File: file, Index: index, Row: row, Disposition: disp},
		hash: r.SpecHash,
		sev:  sev,
	}
}

// latestAuditPolicy is the audit_converge_on of the latest recorded audit
// round, the stricter all when it recorded none or an invalid one.
func latestAuditPolicy(rounds []ReviewRound) string {
	if n := len(rounds); n > 0 && ValidAuditConvergeOn(rounds[n-1].ConvergeOn) {
		return rounds[n-1].ConvergeOn
	}
	return AuditConvergeOnAll
}

// AuditCarriedBlocking returns the audit carried set's rows that hold the loop
// open: not accepted, and blocking under the latest recorded round's policy at
// their carried severity. While it is non-empty the audit is not converged,
// by clean rounds or by its cap.
func AuditCarriedBlocking(specPath string, st *ReviewState) []CarriedRow {
	rows, _ := AuditCarriedSet(specPath, st)
	policy := AuditConvergeOnAll
	if st != nil {
		policy = latestAuditPolicy(st.AuditRounds)
	}
	out := make([]CarriedRow, 0)
	for i := range rows {
		if rows[i].Disposition == CarryAccepted {
			continue
		}
		if policy == AuditConvergeOnBlocking && !auditSeverityBlocking(map[string]any{"severity": rows[i].Severity}) {
			continue
		}
		out = append(out, rows[i])
	}
	return out
}

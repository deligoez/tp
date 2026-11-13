package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	return auditKey{role: AuditRowRole(row), item: rowString(row, "item_id")}
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
	texts := &snapshotItemTexts{specPath: specPath, byRound: make(map[int]map[string]string)}
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
			carryAuditRow(open, texts, k, r, file, j, recorded[j])
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

// listNumberPrefix is a numbered list item's own number at the start of its
// text.
var listNumberPrefix = regexp.MustCompile(`^\s*\d+[.)]\s+`)

// snapshotItemTexts reads, per audit round, the text of each positional item
// its emission snapshot holds, reading a round's snapshot once.
type snapshotItemTexts struct {
	specPath string
	byRound  map[int]map[string]string
}

// text returns the text the round's snapshot holds at a positional id, or
// false when the snapshot is missing or holds no such item.
func (s *snapshotItemTexts) text(round int, id string) (string, bool) {
	items, ok := s.byRound[round]
	if !ok {
		items = make(map[string]string)
		if data, err := os.ReadFile(filepath.Join(ReviewStateDir(s.specPath), snapshotFilename(PhaseAudit, round))); err == nil {
			for _, it := range SpecPositionalItems(strings.Split(string(BlankFrontmatter(data)), "\n")) {
				// A list item's text carries its own number, which a move
				// changes; the item is its words.
				items[it.ID] = strings.TrimSpace(listNumberPrefix.ReplaceAllString(it.Text, ""))
			}
		}
		s.byRound[round] = items
	}
	t, found := items[id]
	return t, found && t != ""
}

// movedTo reports whether the round's snapshot holds text at an id other than
// id.
func (s *snapshotItemTexts) movedTo(round int, id, text string) bool {
	s.text(round, id)
	for other, t := range s.byRound[round] {
		if other != id && t == text {
			return true
		}
	}
	return false
}

// samePositionalItem reports whether a PASS at a positional id in round b
// re-grades the item the id named in round a: their spec texts are equal, or,
// for a table row or list item, round b's snapshot holds round a's text at the
// id, or holds it at no other id because the item itself was repaired. When it
// sits only at another id the item moved, and the id now names another item. A
// finding's ordinal has no text to compare, and a missing snapshot no text to
// read, so either keeps the row unless the spec texts match.
func samePositionalItem(texts *snapshotItemTexts, id string, a, b int, hashA, hashB string) bool {
	if hashA == hashB {
		return true
	}
	if strings.HasPrefix(id, "finding-") {
		return false
	}
	old, ok := texts.text(a, id)
	if !ok {
		return false
	}
	now, readable := texts.text(b, id)
	if !readable {
		return false
	}
	return now == old || !texts.movedTo(b, id, old)
}

// followsMove reports whether a PASS at passID in round b re-grades an open
// table row or list item that sat at openID in round a and moved: round a's
// text at openID is round b's text at passID.
func followsMove(texts *snapshotItemTexts, openID string, a int, passID string, b int) bool {
	if !positionalAuditID(openID) || strings.HasPrefix(openID, "finding-") || openID == passID {
		return false
	}
	old, okA := texts.text(a, openID)
	now, okB := texts.text(b, passID)
	return okA && okB && old == now
}

// carryAuditRow applies one round's worst row under k to the open set.
func carryAuditRow(open map[auditKey]*auditOpen, texts *snapshotItemTexts, k auditKey, r *ReviewRound, file string, index int, row map[string]any) {
	cur := open[k]
	if AuditRowIsPass(row) {
		// A PASS also re-grades an item of the same role that moved here.
		for other, o := range open {
			if other.role == k.role && o.hash != r.SpecHash && followsMove(texts, other.item, o.row.Round, k.item, r.Round) {
				delete(open, other)
			}
		}
		if cur != nil && positionalAuditID(k.item) && !samePositionalItem(texts, k.item, cur.row.Round, r.Round, cur.hash, r.SpecHash) {
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

// AuditCarriedResolveCommand is the acceptance command for one carried audit
// row: its round file and its role:item_id, or its index when the row lacks
// either and so shares its key with every such row of the round.
func AuditCarriedResolveCommand(row *CarriedRow) string {
	selector := AuditRowRole(row.Row) + ":" + rowString(row.Row, "item_id")
	if AuditRowRole(row.Row) == "" || rowString(row.Row, "item_id") == "" {
		selector = fmt.Sprint(row.Index)
	}
	return fmt.Sprintf("tp audit %s --resolve %s wontfix|duplicate \"<evidence>\"", row.File, selector)
}

// AuditCarriedBlockingClause words what stands between an audit and
// convergence when its carried set holds blocking rows, and how each clears.
// Below the cap the next round's prompts list them, and a PASS clears one;
// at the cap no round is admitted, so raising the cap for one is the
// operator's decision. Either way an acceptance clears one, the operator's
// decision, and an epoch cuts them when the spec was rewritten, the
// operator's decision past a blocking row.
func AuditCarriedBlockingClause(rows []CarriedRow, specPath string, atCap bool) string {
	first := &rows[0]
	key := AuditRowRole(first.Row) + ":" + rowString(first.Row, "item_id")
	resolve := AuditCarriedResolveCommand(first)
	pronoun, each := "it", "it"
	clause := fmt.Sprintf("1 carried blocking audit row holds the audit open: %s %s from round %d", first.Severity, key, first.Round)
	if len(rows) > 1 {
		pronoun, each = "them", "each"
		clause = fmt.Sprintf("%d carried blocking audit rows hold the audit open, the first %s %s from round %d (tp audit %s --status lists all)",
			len(rows), first.Severity, key, first.Round, specPath)
	}
	epoch := "; if the spec was rewritten, an epoch cuts " + pronoun + " instead: tp review " + specPath +
		" --rewritten \"<evidence>\", the operator's decision " + EscalateRewritePastBlocking
	orphaned := orphanedAuditRows(rows, specPath)
	switch {
	case orphaned == len(rows):
		// No prompt reaches a row whose role the panel no longer holds, so
		// no round re-grades it, under the cap or past it.
		return clause + fmt.Sprintf("; no role of the next round can re-grade %s, since the audit panel no longer holds its role: accept %s, the operator's decision (%s) — %s",
			pronoun, each, EscalateAcceptFinding, resolve) + epoch
	case orphaned > 0:
		clause += fmt.Sprintf("; %d of them belong to a role the audit panel no longer holds, which no round re-grades", orphaned)
	}
	if atCap {
		return clause + fmt.Sprintf("; the round cap admits no round to re-grade %s: accept %s, the operator's decision (%s) — %s; or raise audit_max_rounds for a round that re-grades %s, the operator's decision (%s)",
			pronoun, each, EscalateAcceptFinding, resolve, pronoun, EscalateRaiseAuditCap) + epoch
	}
	return clause + fmt.Sprintf("; the next round's prompts list %s to re-grade, and only a PASS clears it: run it — tp audit %s --record <file>; or accept %s, the operator's decision (%s) — %s",
		pronoun, specPath, each, EscalateAcceptFinding, resolve) + epoch
}

// orphanedAuditRows counts the rows whose role the spec's resolved audit panel
// does not hold: no prompt of a later round reaches them. A panel that cannot
// be resolved counts none, so the clause falls back to naming the next round.
func orphanedAuditRows(rows []CarriedRow, specPath string) int {
	panel, err := ResolveRolePanel(specPath, PhaseAuditors)
	if err != nil {
		return 0
	}
	active := make(map[string]bool, len(panel.Roles))
	for i := range panel.Roles {
		active[panel.Roles[i].ID] = true
	}
	n := 0
	for i := range rows {
		if !active[AuditRowRole(rows[i].Row)] {
			n++
		}
	}
	return n
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

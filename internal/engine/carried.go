package engine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// findingPrefixLen is how much of a finding's text its identity reads.
const findingPrefixLen = 80

// FindingIdentityKey is a review finding's identity across rounds: its
// category, location and the first 80 bytes of its text. Two different
// findings at one location stay two rows; the same finding re-reported in a
// later round is one.
func FindingIdentityKey(category, location, finding string) string {
	prefix := finding
	if len(prefix) > findingPrefixLen {
		prefix = prefix[:findingPrefixLen]
	}
	return category + "::" + location + "::" + prefix
}

// The carry's three dispositions (spec/backlog/carried-rows-block.md §2.2).
const (
	CarryUnresolved = "unresolved"
	CarryAccepted   = "accepted"
	CarryResolved   = "resolved"
)

// CarryDisposition sorts a finding the way the carry lists it: fixed is
// resolved; wontfix or duplicate with non-blank evidence is accepted; anything
// else — no disposition, a blank-evidence acceptance, an unknown status — is
// unresolved. hasResolved says whether the row carries a disposition at all.
func CarryDisposition(hasResolved bool, status, evidence string) string {
	switch {
	case !hasResolved:
		return CarryUnresolved
	case status == "fixed":
		return CarryResolved
	case (status == "wontfix" || status == "duplicate") && strings.TrimSpace(evidence) != "":
		return CarryAccepted
	default:
		return CarryUnresolved
	}
}

// The two states a blocking carried row is in (§2.6): still unresolved, or
// marked fixed and waiting for a round whose emission listed the fix to be
// recorded.
const (
	CarriedStateUnresolved      = "unresolved"
	CarriedStateFixedUnverified = "fixed-unverified"
)

// CarriedRow is one finding of the carried set: the row as its latest round
// recorded it, where it lives, and its disposition.
type CarriedRow struct {
	Round int
	// File is the round file joined to the spec's state directory, the
	// positional `tp review <file> --resolve <index>` takes.
	File        string
	Index       int
	Row         map[string]any
	Disposition string
	// Severity is the highest severity among the finding's occurrences since
	// its last resolving disposition: a later copy graded lower does not
	// downgrade it.
	Severity string
	// Verified is, for a row whose latest disposition is fixed, whether a
	// review round emitted after the fix has been recorded.
	Verified bool
	// Earlier is whether the row's latest occurrence is in a round before the
	// latest recorded one: a row no further round will raise again.
	Earlier bool
}

// State is the blocking state a blocking carried row is in.
func (r *CarriedRow) State() string {
	if r.Disposition == CarryResolved {
		return CarriedStateFixedUnverified
	}
	return CarriedStateUnresolved
}

// CarriedRowJSON is the bounded shape a payload names a carried row in.
type CarriedRowJSON struct {
	Round    int    `json:"round"`
	File     string `json:"file"`
	Index    int    `json:"index"`
	Severity string `json:"severity"`
	Location string `json:"location"`
	Finding  string `json:"finding"`
	State    string `json:"state"`
}

// CarriedRowsShown bounds how many carried rows a payload lists in full; the
// count beside them is always the whole.
const CarriedRowsShown = 5

func rowString(row map[string]any, key string) string {
	s, _ := row[key].(string)
	return s
}

// JSON renders r for a payload, its finding cut to the identity's prefix.
func (r *CarriedRow) JSON() CarriedRowJSON {
	finding := rowString(r.Row, "finding")
	if len(finding) > findingPrefixLen {
		finding = finding[:RuneBoundaryAtOrBefore(finding, findingPrefixLen)] + "..."
	}
	return CarriedRowJSON{
		Round: r.Round, File: r.File, Index: r.Index,
		Severity: r.Severity,
		Location: rowString(r.Row, "location"),
		Finding:  finding,
		State:    r.State(),
	}
}

// CarriedRowsJSON renders at most CarriedRowsShown of rows.
func CarriedRowsJSON(rows []CarriedRow) []CarriedRowJSON {
	out := make([]CarriedRowJSON, 0, min(len(rows), CarriedRowsShown))
	for i := range rows[:min(len(rows), CarriedRowsShown)] {
		out = append(out, rows[i].JSON())
	}
	return out
}

// CarriedEarlier returns the rows whose latest occurrence is in a round
// before the latest recorded one: what a payload lists, since the latest
// round's own rows are that round's to dispose of.
func CarriedEarlier(rows []CarriedRow) []CarriedRow {
	out := make([]CarriedRow, 0, len(rows))
	for i := range rows {
		if rows[i].Earlier {
			out = append(out, rows[i])
		}
	}
	return out
}

// CarriedFixedUnverified counts the rows marked fixed and awaiting a
// verifying round.
func CarriedFixedUnverified(rows []CarriedRow) int {
	n := 0
	for i := range rows {
		if rows[i].State() == CarriedStateFixedUnverified {
			n++
		}
	}
	return n
}

// ReviewCarriedSet is the one carried set (spec/backlog/carried-rows-block.md
// §2.1): every finding the next review emission lists from earlier rounds. It
// reads the rounds after the newest epoch (RoundsSinceEpoch), keeps each
// finding's latest occurrence by FindingIdentityKey — so a row resolved in a
// later round is not held open by an earlier copy — and sorts each by
// CarryDisposition. Rows are in the order of their latest occurrence. missing
// lists the carried rounds whose file could not be read.
//
// The emission's "UNRESOLVED … DO NOT re-report" listing and review
// convergence both read it, so a row the roles are told to stay silent about
// is the same row convergence waits on.
func ReviewCarriedSet(specPath string, st *ReviewState) (rows []CarriedRow, missing []ReviewRound) {
	carried := RoundsSinceEpoch(st, PhaseReview)
	all := make([]CarriedRow, 0)
	for i := range carried {
		r := &carried[i]
		recorded, found := LoadRoundRows(specPath, r)
		if !found {
			missing = append(missing, *r)
			continue
		}
		file := filepath.Join(ReviewStateDir(specPath), r.File)
		for j, row := range recorded {
			resolved, has := row["resolved"].(map[string]any)
			all = append(all, CarriedRow{
				Round: r.Round, File: file, Index: j, Row: row,
				Disposition: CarryDisposition(has, rowString(resolved, "status"), rowString(resolved, "evidence")),
			})
		}
	}
	latest := latestRecordedRound(reviewRoundsOf(st))
	kept := latestByIdentity(all)
	for i := range kept {
		kept[i].Verified = fixVerified(&kept[i], latest)
		kept[i].Earlier = kept[i].Round < latest
	}
	return kept, missing
}

// latestRecordedRound is the highest recorded round number, 0 when none.
func latestRecordedRound(rounds []ReviewRound) int {
	n := 0
	for i := range rounds {
		n = max(n, rounds[i].Round)
	}
	return n
}

// fixVerified reports whether a row's fixed disposition has been verified
// (§2.6): a review round numbered above the disposition's resolved_after_round
// — the review rounds recorded or emitted when it was written — is recorded.
// A disposition with no recorded order counts as verified if a round later
// than its row's round is recorded. Any other disposition is not a fix and
// reports false.
func fixVerified(r *CarriedRow, latest int) bool {
	if r.Disposition != CarryResolved {
		return false
	}
	resolved, _ := r.Row["resolved"].(map[string]any)
	if after, ok := resolved["resolved_after_round"].(float64); ok {
		return float64(latest) > after
	}
	return latest > r.Round
}

// latestByIdentity keeps each finding's last occurrence in rows, in the order
// of those last occurrences, graded at the highest severity among its
// occurrences since its last resolving disposition (resolved or accepted).
func latestByIdentity(rows []CarriedRow) []CarriedRow {
	identity := func(row map[string]any) string {
		return FindingIdentityKey(rowString(row, "category"), rowString(row, "location"), rowString(row, "finding"))
	}
	last := make(map[string]int, len(rows))
	severity := make(map[string]string, len(rows))
	resolving := make(map[string]bool, len(rows))
	for i := range rows {
		key := identity(rows[i].Row)
		last[key] = i
		sev := rowString(rows[i].Row, "severity")
		if prev, seen := severity[key]; seen && !resolving[key] && severityRank(prev) < severityRank(sev) {
			sev = prev
		}
		severity[key] = sev
		resolving[key] = rows[i].Disposition != CarryUnresolved
	}
	kept := make([]CarriedRow, 0, len(last))
	for i := range rows {
		key := identity(rows[i].Row)
		if last[key] == i {
			row := rows[i]
			row.Severity = severity[key]
			kept = append(kept, row)
		}
	}
	return kept
}

// severityRank orders review severities from the most severe: critical, high,
// anything tp cannot grade (which blocks), then medium and low.
func severityRank(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 3
	case "low":
		return 4
	}
	return 2
}

// ReviewCarriedBlocking returns the carried set's rows that block under
// convergeOn — every one under all, the critical, high and ungradable ones
// under blocking — and are unresolved, or fixed with no verifying round
// recorded yet. While it is non-empty the review loop is not converged.
func ReviewCarriedBlocking(specPath string, st *ReviewState, convergeOn string) []CarriedRow {
	rows, _ := ReviewCarriedSet(specPath, st)
	return carriedBlockingOf(rows, convergeOn)
}

func carriedBlockingOf(rows []CarriedRow, convergeOn string) []CarriedRow {
	out := make([]CarriedRow, 0)
	for i := range rows {
		switch {
		case rows[i].Disposition == CarryUnresolved:
		case rows[i].Disposition == CarryResolved && !rows[i].Verified:
		default:
			continue
		}
		if convergeOn == ReviewConvergeOnAll || reviewSeverityBlocking(map[string]any{"severity": rows[i].Severity}) {
			out = append(out, rows[i])
		}
	}
	return out
}

// CarriedBlockingClause words what stands between a review loop and
// convergence when its carried set holds blocking rows, and how each kind
// clears. An unresolved row clears by its own disposition in its round file
// or, when the text was rewritten, an epoch, which past a blocking row is the
// operator's decision. A row marked fixed clears when a round that reads the
// fix is recorded; at the cap no round is admitted, so raising the cap for one
// is the operator's decision, and fixed is never offered as an ending.
func CarriedBlockingClause(rows []CarriedRow, specPath string, atCap bool) string {
	unresolved, fixed := make([]CarriedRow, 0, len(rows)), 0
	for i := range rows {
		if rows[i].State() == CarriedStateFixedUnverified {
			fixed++
			continue
		}
		unresolved = append(unresolved, rows[i])
	}
	parts := make([]string, 0, 2)
	if len(unresolved) > 0 {
		parts = append(parts, unresolvedClause(unresolved, specPath, atCap))
	}
	if fixed > 0 {
		parts = append(parts, fixedClause(fixed, specPath, atCap))
	}
	return strings.Join(parts, "; ")
}

func unresolvedClause(rows []CarriedRow, specPath string, atCap bool) string {
	noun, pronoun := "finding stands", "it"
	if len(rows) > 1 {
		noun, pronoun = "findings stand", "them"
	}
	// At the cap a fix cannot end the loop, since no round may verify it.
	statuses, fixNote := "fixed|wontfix|duplicate", ""
	if atCap {
		statuses = "wontfix|duplicate"
		fixNote = "; a fix needs a round to verify it, which the cap admits only once the operator raises it (" + EscalateRaiseReviewCap + ")"
	}
	first := &rows[0]
	clause := fmt.Sprintf("%d carried blocking %s unresolved, and the next round's roles are told not to re-report %s: resolve each — the first, %s %s in round %d: tp review %s --resolve %d %s \"<evidence>\"",
		len(rows), noun, pronoun, first.Severity, rowString(first.Row, "location"), first.Round, first.File, first.Index, statuses) + fixNote
	if len(rows) > 1 {
		clause += fmt.Sprintf("; %d more are listed by tp review %s --status", len(rows)-1, specPath)
	}
	return clause + "; accepting a critical or high finding is the operator's decision. If the spec was rewritten, an epoch cuts " + pronoun + " instead: tp review " +
		specPath + " --rewritten \"<evidence>\", the operator's decision " + EscalateRewritePastBlocking
}

func fixedClause(n int, specPath string, atCap bool) string {
	noun := "finding marked fixed awaits"
	if n > 1 {
		noun = "findings marked fixed await"
	}
	clause := fmt.Sprintf("%d carried blocking %s a review round that reads the fix", n, noun)
	if atCap {
		return clause + "; the round cap admits none, so raising review_max_rounds for one is the operator's decision (" + EscalateRaiseReviewCap + ")"
	}
	return clause + ": run the next review round — tp review " + specPath + ", then tp review " + specPath + " --record <file>"
}

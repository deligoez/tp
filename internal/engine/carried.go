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
}

// CarriedRowJSON is the bounded shape a payload names a carried row in.
type CarriedRowJSON struct {
	Round    int    `json:"round"`
	File     string `json:"file"`
	Index    int    `json:"index"`
	Severity string `json:"severity"`
	Location string `json:"location"`
	Finding  string `json:"finding"`
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
		Severity: rowString(r.Row, "severity"),
		Location: rowString(r.Row, "location"),
		Finding:  finding,
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
	return latestByIdentity(all), missing
}

// latestByIdentity keeps each finding's last occurrence in rows, in the order
// of those last occurrences.
func latestByIdentity(rows []CarriedRow) []CarriedRow {
	seen := make(map[string]bool, len(rows))
	kept := make([]CarriedRow, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i].Row
		key := FindingIdentityKey(rowString(row, "category"), rowString(row, "location"), rowString(row, "finding"))
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, rows[i])
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept
}

// ReviewCarriedBlocking returns the carried set's unresolved rows that block
// under convergeOn: every one under all, the critical, high and ungradable
// ones under blocking. While it is non-empty the review loop is not converged.
func ReviewCarriedBlocking(specPath string, st *ReviewState, convergeOn string) []CarriedRow {
	rows, _ := ReviewCarriedSet(specPath, st)
	return carriedBlockingOf(rows, convergeOn)
}

func carriedBlockingOf(rows []CarriedRow, convergeOn string) []CarriedRow {
	out := make([]CarriedRow, 0)
	for i := range rows {
		if rows[i].Disposition != CarryUnresolved {
			continue
		}
		if convergeOn == ReviewConvergeOnAll || reviewSeverityBlocking(rows[i].Row) {
			out = append(out, rows[i])
		}
	}
	return out
}

// CarriedBlockingClause words what stands between a review loop and
// convergence when its carried set holds blocking rows, and how each clears:
// its own disposition in its round file, or, when the text was rewritten, an
// epoch, which past a blocking row is the operator's decision. It names the
// first row's command and counts the rest, since --status lists them.
func CarriedBlockingClause(rows []CarriedRow, specPath string) string {
	if len(rows) == 0 {
		return ""
	}
	noun, pronoun := "finding stands", "it"
	if len(rows) > 1 {
		noun, pronoun = "findings stand", "them"
	}
	first := &rows[0]
	clause := fmt.Sprintf("%d carried blocking %s unresolved, and the next round's roles are told not to re-report %s: resolve each — tp review %s --resolve %d fixed|wontfix|duplicate \"<evidence>\" (%s %s, round %d)",
		len(rows), noun, pronoun, first.File, first.Index, rowString(first.Row, "severity"), rowString(first.Row, "location"), first.Round)
	if len(rows) > 1 {
		clause += fmt.Sprintf(", and %d more listed by tp review %s --status", len(rows)-1, specPath)
	}
	return clause + "; accepting a critical or high finding is the operator's decision. If the spec was rewritten, an epoch cuts them instead: tp review " +
		specPath + " --rewritten \"<evidence>\", the operator's decision " + EscalateRewritePastBlocking
}

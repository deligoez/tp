package engine

import (
	"sort"
)

// RoleSpecCoverage is the auditor role id that carries the spec-derived
// checklist, and therefore the only role in the corpus that measures spec
// conformance. §2.2 reports it first in role_streaks and §2.3 reports its
// streak again as a named top-level field, so the id is a constant here rather
// than a literal repeated at each site.
const RoleSpecCoverage = "spec-coverage"

// RoleStreak is one entry of the role_streaks array (§2.2): a role appearing in
// the latest recorded audit round, the length of its trailing run of clean
// rounds, and its non-PASS row count in that latest round.
//
// Every entry has rows in the latest round, so Open == 0 and
// ConsecutiveClean >= 1 are the same condition and a role holding open rows
// always carries a streak of 0. Both fields are reported on every entry anyway:
// a per-role array with a hole in it is worse to parse than one with a
// constant, and the pair is informative on the quiet roles, where Open gives
// one bit and ConsecutiveClean gives its length.
type RoleStreak struct {
	Role             string `json:"role"`
	ConsecutiveClean int    `json:"consecutive_clean"`
	Open             int    `json:"open"`
}

// ComputeAuditRoleStreaks computes §2.2's role_streaks over the recorded audit
// rounds of one spec, newest last, as ReviewState.AuditRounds stores them.
//
// A role is clean in a round when it has at least one row in that round and
// every one of its rows is PASS. A role with no rows in a round is not clean in
// it: the role was not measured, so cleanliness cannot be claimed for it, and
// the round therefore ends that role's streak instead of being skipped over. A
// round that contributes no rows by any of §2.1's four causes ends every
// streak at once, as does a round whose rows all carry no role. Both are the
// conservative direction — they reset the signal toward "keep auditing" rather
// than toward "you may ship".
//
// The returned entries are the roles of the latest recorded round alone — the
// panel the current decision rests on, not every role ever seen — with
// spec-coverage first when present and the rest in ascending byte order, the
// ordinary Go string comparison. §2.1 makes a non-lowercase id reachable and
// requires it to stay distinct, so the comparison must be the one that keeps it
// distinct rather than a case-insensitive or locale-aware sort.
//
// The slice is always non-nil, so it serializes as an emitted [] rather than a
// null in the four states that reach it: no recorded round at all, a latest
// round contributing no rows, a latest round recording zero rows, and a latest
// round whose every row carries no role.
//
// The walk starts at the latest round and stops as soon as no further round can
// change a reported value: when every latest-round role's streak has closed,
// when a round contributes no rows, or at the start of the recorded history.
// Stopping on the first no-rows round is what keeps §2.1's advisory to at most
// one per invocation.
//
// latestRows is the latest round's rows as the walk already read them, nil when
// that round contributed none. It is returned rather than left to be re-read
// because §2.4's per-row facts about that round — the open rows carrying no
// role — are not derivable from the streaks, and a second ReadAuditRoundRows
// call over the same entry would both re-parse the file and risk re-firing the
// advisory this walk fires at most once.
func ComputeAuditRoleStreaks(specPath string, rounds []ReviewRound) (streaks []RoleStreak, latestRows []map[string]any) {
	streaks = make([]RoleStreak, 0)
	if len(rounds) == 0 {
		return streaks, nil
	}

	// The latest round's own stored hash is the panel now in force: every
	// earlier round is judged against it, and the latest round is judged
	// against itself, so only its empty case can close the walk here.
	latestHash := rounds[len(rounds)-1].RolesHash
	rows, ok := ReadAuditRoundRows(specPath, &rounds[len(rounds)-1], latestHash)
	if !ok {
		return streaks, nil
	}
	latestRows = rows

	latestOpen := auditRoundOpenByRole(rows)
	consecutive := make(map[string]int, len(latestOpen))
	streakOpen := make(map[string]struct{}, len(latestOpen))
	for role, open := range latestOpen {
		if open == 0 {
			consecutive[role] = 1
			streakOpen[role] = struct{}{}
		}
	}

	for i := len(rounds) - 2; i >= 0 && len(streakOpen) > 0; i-- {
		earlierRows, contributed := ReadAuditRoundRows(specPath, &rounds[i], latestHash)
		if !contributed {
			break
		}
		earlierOpen := auditRoundOpenByRole(earlierRows)
		for role := range streakOpen {
			if open, measured := earlierOpen[role]; measured && open == 0 {
				consecutive[role]++
				continue
			}
			delete(streakOpen, role)
		}
	}

	for role, open := range latestOpen {
		streaks = append(streaks, RoleStreak{Role: role, ConsecutiveClean: consecutive[role], Open: open})
	}
	sort.Slice(streaks, func(i, j int) bool {
		a, b := streaks[i].Role, streaks[j].Role
		if (a == RoleSpecCoverage) != (b == RoleSpecCoverage) {
			return a == RoleSpecCoverage
		}
		return a < b
	})
	return streaks, latestRows
}

// auditRoundOpenByRole tallies one round's rows by role id: every role with at
// least one row in the round is a key, mapped to how many of its rows are
// non-PASS. A role is clean in the round exactly when it is a key with the
// value 0, which keeps "measured and all-PASS" distinguishable from "not
// measured" — the distinction a role absent from a round turns on.
//
// Rows carrying no role are excluded, since they attribute to nothing; a round
// whose rows all carry no role therefore tallies empty and closes every streak.
func auditRoundOpenByRole(rows []map[string]any) map[string]int {
	open := make(map[string]int, len(rows))
	for _, row := range rows {
		role := AuditRowRole(row)
		if role == "" {
			continue
		}
		if _, seen := open[role]; !seen {
			open[role] = 0
		}
		if !AuditRowIsPass(row) {
			open[role]++
		}
	}
	return open
}

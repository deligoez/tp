package engine

import "strings"

// nonBlockingReviewSeverities is the fixed set of below-blocking severities
// (§4.1). The blocking severities are critical and high; classification is by
// exclusion so that every other value — a missing or out-of-vocabulary
// severity included — is treated conservatively as blocking. The sets are
// fixed in this release, not a knob.
var nonBlockingReviewSeverities = map[string]bool{
	"medium": true,
	"low":    true,
}

// reviewSeverityBlocking reports whether a finding's severity blocks a clean
// round under the blocking policy (§4.1). Severity is normalized to lower case
// before classifying; medium and low are non-blocking, and anything else —
// critical, high, a missing severity, or an out-of-vocabulary value — is
// treated conservatively as blocking so an ill-formed severity from an LLM
// reviewer never lets a genuine defect count toward convergence.
func reviewSeverityBlocking(row map[string]any) bool {
	sev, _ := row["severity"].(string)
	norm := strings.ToLower(strings.TrimSpace(sev))
	return !nonBlockingReviewSeverities[norm]
}

// reviewFindingResolvedAway reports whether a recorded finding is out of the
// surviving set: resolved wontfix or duplicate with non-empty evidence (§3.4,
// §4.3). The resolution is read live from the round's recorded row, so a
// later --resolve/--resolve-all edit re-evaluates the round without
// re-recording.
func reviewFindingResolvedAway(row map[string]any) bool {
	resolved, ok := row["resolved"].(map[string]any)
	if !ok {
		return false
	}
	status, _ := resolved["status"].(string)
	evidence, _ := resolved["evidence"].(string)
	if strings.TrimSpace(evidence) == "" {
		return false
	}
	return status == "wontfix" || status == "duplicate"
}

// ReviewRoundClean recomputes a recorded review round's cleanliness live from
// its recorded findings under review_converge_on (§3.4, §4.1). The surviving
// set is the round's recorded findings minus those resolved wontfix/duplicate
// (with evidence). Under "all" a round is clean when zero findings survive;
// under "blocking" (the default) it is clean when no surviving finding is of a
// blocking severity. Because both the surviving set and its severities are read
// live from the round's recorded findings file, switching review_converge_on or
// resolving a finding wontfix after the round was recorded both re-evaluate the
// round's cleanliness without re-recording. A round whose recorded findings
// file is missing falls back to the stored clean flag so round arithmetic is
// preserved. This is a review-only predicate; audit convergence is unaffected.
func ReviewRoundClean(specPath string, entry *ReviewRound, convergeOn string) bool {
	rows, found := LoadRoundRows(specPath, entry)
	if !found {
		return entry.Clean
	}
	survivingBlocking := 0
	surviving := 0
	for _, row := range rows {
		if reviewFindingResolvedAway(row) {
			continue
		}
		surviving++
		if reviewSeverityBlocking(row) {
			survivingBlocking++
		}
	}
	if convergeOn == ReviewConvergeOnAll {
		return surviving == 0
	}
	// blocking policy (the default): clean when no surviving finding blocks.
	return survivingBlocking == 0
}

// ReviewRoundNonBlockingOpen counts a recorded review round's surviving
// non-blocking findings — those left in the surviving set (not resolved
// wontfix/duplicate with evidence) whose severity is a below-blocking value
// (medium/low) under the blocking policy (§4.2). This is the count that
// distinguishes an accepted-open clean round (clean only because every survivor
// is below the blocking severities) from a survivor-free clean round.
//
// Under review_converge_on=all nothing is non-blocking — any survivor blocks —
// so the count is always 0; and because a clean "all" round has zero survivors
// anyway, callers gating on (clean && count>0) never emit for it. A missing or
// out-of-vocabulary severity is blocking (reviewSeverityBlocking), so such a
// survivor is never counted here. A round whose recorded findings file is
// missing yields 0. Read live from the round's recorded findings, mirroring
// ReviewRoundClean, so a later --resolve or converge-on switch re-evaluates it
// without re-recording. Review-only; audit convergence has no non-blocking
// notion.
func ReviewRoundNonBlockingOpen(specPath string, entry *ReviewRound, convergeOn string) int {
	if convergeOn == ReviewConvergeOnAll {
		return 0
	}
	rows, found := LoadRoundRows(specPath, entry)
	if !found {
		return 0
	}
	n := 0
	for _, row := range rows {
		if reviewFindingResolvedAway(row) {
			continue
		}
		if !reviewSeverityBlocking(row) {
			n++
		}
	}
	return n
}

// ReviewConsecutiveClean counts trailing review rounds that are clean under the
// current review_converge_on, recomputed live per round (§3.4). It mirrors
// ConsecutiveClean but is severity-aware; audit sequences keep using
// ConsecutiveClean.
func ReviewConsecutiveClean(specPath string, rounds []ReviewRound, convergeOn string) int {
	n := 0
	for i := len(rounds) - 1; i >= 0; i-- {
		if !ReviewRoundClean(specPath, &rounds[i], convergeOn) {
			break
		}
		n++
	}
	return n
}

// ReviewConverged reports review convergence: enough trailing severity-aware
// clean rounds (§3.4) and a spec unchanged since the last recorded round. It
// mirrors Converged but consumes review_converge_on; audit convergence keeps
// using Converged.
func ReviewConverged(specPath string, rounds []ReviewRound, requiredCleanRounds int, currentHash, convergeOn string) bool {
	return ReviewConsecutiveClean(specPath, rounds, convergeOn) >= requiredCleanRounds && !StateStale(rounds, currentHash)
}

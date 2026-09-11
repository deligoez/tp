package engine

// DoneByConverged and DoneByCap name why a review or audit loop has ended:
// enough clean rounds, or the round cap reached with every finding of the
// latest round carrying a disposition. A loop that ends by its cap has not been
// graded clean twice; the payloads that report it say which one held.
const (
	DoneByConverged = "converged"
	DoneByCap       = "cap"
)

// DefaultMaxRounds is the built-in review_max_rounds and audit_max_rounds.
const DefaultMaxRounds = 3

// LoopDone is the one verdict import, `--status --check`, next_action and
// tp resume read, so none of them can keep a loop open that another has let
// end. Done is the verdict and By its reason; the rest describe the latest
// round when the cap is reached and are zero otherwise.
type LoopDone struct {
	Done        bool
	By          string
	CapReached  bool
	Undisposed  int  // latest round's findings with no disposition that counts
	FixedAtCap  int  // latest round's findings dispositioned fixed, which no round re-read
	StaleWaived bool // the spec changed after the latest round, which ended the loop anyway
}

// RowDispositioned reports whether a finding row carries a disposition that
// counts: fixed, or wontfix/duplicate with non-empty evidence. A wontfix whose
// evidence is blank clears nothing (findingResolvedAway), so it cannot end a
// loop either.
func RowDispositioned(row map[string]any) bool {
	if findingResolvedAway(row) {
		return true
	}
	resolved, ok := row["resolved"].(map[string]any)
	if !ok {
		return false
	}
	status, _ := resolved["status"].(string)
	return status == "fixed"
}

func rowFixed(row map[string]any) bool {
	resolved, ok := row["resolved"].(map[string]any)
	if !ok {
		return false
	}
	status, _ := resolved["status"].(string)
	return status == "fixed"
}

// ReviewLoopDone reports whether the review loop has ended. Every review row
// is a finding.
func ReviewLoopDone(specPath string, rounds []ReviewRound, requiredClean, maxRounds int, currentHash, convergeOn string) LoopDone {
	converged := ReviewConverged(specPath, rounds, requiredClean, currentHash, convergeOn)
	return loopDone(converged, specPath, rounds, maxRounds, currentHash, func(map[string]any) bool { return true })
}

// AuditLoopDone reports whether the audit loop has ended. Only non-PASS rows
// are findings, so a PASS row needs no disposition. Each round is graded live
// under its own recorded policy (AuditRoundClean).
func AuditLoopDone(specPath string, rounds []ReviewRound, requiredClean, maxRounds int, currentHash string) LoopDone {
	converged := Converged(LiveAuditRounds(specPath, rounds), requiredClean, currentHash)
	return loopDone(converged, specPath, rounds, maxRounds, currentHash, func(row map[string]any) bool { return !AuditRowIsPass(row) })
}

func loopDone(converged bool, specPath string, rounds []ReviewRound, maxRounds int, currentHash string, isFinding func(map[string]any) bool) LoopDone {
	if converged {
		return LoopDone{Done: true, By: DoneByConverged}
	}
	if maxRounds <= 0 || len(rounds) < maxRounds {
		return LoopDone{}
	}
	d := LoopDone{CapReached: true}
	rows, found := LoadRoundRows(specPath, &rounds[len(rounds)-1])
	if !found {
		return d
	}
	for _, row := range rows {
		if !isFinding(row) {
			continue
		}
		switch {
		case rowFixed(row):
			d.FixedAtCap++
		case !RowDispositioned(row):
			d.Undisposed++
		}
	}
	if d.Undisposed == 0 {
		d.Done, d.By = true, DoneByCap
		d.StaleWaived = StateStale(rounds, currentHash)
	}
	return d
}

// AuditRoundClean is a recorded audit round's live verdict. A round stamped
// clean stays clean: a disposition can only take rows out of the graded set.
// A round stamped unclean is re-graded from its recorded rows with the rows
// accepted wontfix/duplicate with evidence taken out, under the policy the
// round itself was recorded with — so a disposition written after the round
// clears it, while changing audit_converge_on does not reach back into it. A
// round recorded before the policy was stored is graded under the stricter
// `all`; a round whose file is missing keeps its stored flag.
func AuditRoundClean(specPath string, entry *ReviewRound) bool {
	if entry.Clean {
		return true
	}
	rows, found := LoadRoundRows(specPath, entry)
	if !found {
		return false
	}
	policy := entry.ConvergeOn
	if !ValidAuditConvergeOn(policy) {
		policy = AuditConvergeOnAll
	}
	return AuditRowsCleanLive(rows, policy)
}

// AuditRowsCleanLive grades rows as AuditRowsClean does after dropping the
// rows accepted wontfix/duplicate with evidence. --record stamps with it, so
// a row that arrives already accepted does not dirty the round it is recorded
// in, matching review.
func AuditRowsCleanLive(rows []map[string]any, convergeOn string) bool {
	live := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if !findingResolvedAway(row) {
			live = append(live, row)
		}
	}
	return AuditRowsClean(live, convergeOn)
}

// LiveAuditRounds returns a copy of rounds with each Clean flag replaced by
// AuditRoundClean, so ConsecutiveClean and Converged read the live verdict.
// The caller's slice is not written through.
func LiveAuditRounds(specPath string, rounds []ReviewRound) []ReviewRound {
	live := make([]ReviewRound, len(rounds))
	copy(live, rounds)
	for i := range live {
		live[i].Clean = AuditRoundClean(specPath, &live[i])
	}
	return live
}

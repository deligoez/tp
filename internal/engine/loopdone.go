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
	Done       bool
	By         string
	CapReached bool
	Undisposed int // latest round's findings with no disposition that counts
	FixedAtCap int // latest round's non-blocking findings dispositioned fixed, which no round re-read
	// BlockingFixedAtCap counts the latest round's blocking findings marked
	// fixed at the cap. fixed says the text or code changed, and at the cap no
	// round will re-read it, so for a blocking finding it would be an
	// acceptance nobody made — and a unit may write fixed under TP_UNATTENDED.
	// Such a row keeps the loop open until the operator accepts it with
	// evidence or raises the cap for a verification round.
	BlockingFixedAtCap int
	StaleWaived        bool // the spec changed after the latest round, which ended the loop anyway
}

// RowDispositioned reports whether a finding row carries a disposition that
// counts: fixed, or wontfix/duplicate with non-empty evidence. A wontfix whose
// evidence is blank clears nothing (findingResolvedAway), so it cannot end a
// loop either.
func RowDispositioned(row map[string]any) bool {
	return findingResolvedAway(row) || rowFixed(row)
}

// RowAccepted reports whether a finding row was accepted: wontfix or
// duplicate with non-blank evidence, the one disposition that takes a row out
// of the graded set (findingResolvedAway). fixed is not an acceptance.
func RowAccepted(row map[string]any) bool {
	return findingResolvedAway(row)
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
// is a finding; a finding blocks when it would hold a round open under the
// resolved review_converge_on.
func ReviewLoopDone(specPath string, rounds []ReviewRound, requiredClean, maxRounds int, currentHash, convergeOn string) LoopDone {
	converged := ReviewConverged(specPath, rounds, requiredClean, currentHash, convergeOn)
	blocking := func(row map[string]any) bool {
		return convergeOn == ReviewConvergeOnAll || ReviewRowBlocking(row)
	}
	return loopDone(converged, specPath, rounds, maxRounds, currentHash, func(map[string]any) bool { return true }, blocking)
}

// AuditLoopDone reports whether the audit loop has ended. Only non-PASS rows
// are findings, so a PASS row needs no disposition. Each round is graded live
// under its own recorded policy (AuditRoundClean), and a finding of the latest
// round blocks when that round's policy would hold it open.
func AuditLoopDone(specPath string, rounds []ReviewRound, requiredClean, maxRounds int, currentHash string) LoopDone {
	converged := Converged(LiveAuditRounds(specPath, rounds), requiredClean, currentHash)
	policy := AuditConvergeOnAll
	if n := len(rounds); n > 0 && ValidAuditConvergeOn(rounds[n-1].ConvergeOn) {
		policy = rounds[n-1].ConvergeOn
	}
	blocking := func(row map[string]any) bool {
		return policy != AuditConvergeOnBlocking || auditSeverityBlocking(row)
	}
	return loopDone(converged, specPath, rounds, maxRounds, currentHash, func(row map[string]any) bool { return !AuditRowIsPass(row) }, blocking)
}

func loopDone(converged bool, specPath string, rounds []ReviewRound, maxRounds int, currentHash string, isFinding, isBlocking func(map[string]any) bool) LoopDone {
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
		case rowFixed(row) && isBlocking(row):
			d.BlockingFixedAtCap++
		case rowFixed(row):
			d.FixedAtCap++
		case !RowDispositioned(row):
			d.Undisposed++
		}
	}
	if d.Undisposed == 0 && d.BlockingFixedAtCap == 0 {
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

// AuditRowsCleanLive grades the round's open findings (findingOpen) as
// AuditRowsClean does: the rows accepted wontfix/duplicate with evidence are
// dropped, and so are PASS rows, which AuditRowsClean never grades. --record
// stamps with it, so a row that arrives already accepted does not dirty the
// round it is recorded in, matching review.
func AuditRowsCleanLive(rows []map[string]any, convergeOn string) bool {
	live := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if findingOpen(PhaseAudit, row) {
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

package engine

// Epoch is one `tp review <spec> --rewritten "<evidence>"` (spec/backlog/reconcile.md):
// the spec was rewritten, and the rounds recorded so far describe a document
// that no longer exists. It holds a position in each phase, because review and
// audit number their rounds independently and both phases' readers need a cut
// point: the rounds of a phase numbered at or below its position were recorded
// before the epoch.
//
// An epoch changes prompts only. Nothing recorded changes, no disposition is
// written, convergence still reads the trailing clean rounds and the round cap
// still counts every recorded round.
type Epoch struct {
	AfterReviewRound int    `json:"after_review_round"`
	AfterAuditRound  int    `json:"after_audit_round"`
	Evidence         string `json:"evidence"`
	At               string `json:"at"`
}

// EpochCut returns the newest epoch's position for phase (PhaseReview or
// PhaseAudit): its rounds numbered at or below it were recorded before the
// epoch. 0 when there is no epoch, which cuts nothing.
func EpochCut(st *ReviewState, phase string) int {
	if st == nil || len(st.Epochs) == 0 {
		return 0
	}
	newest := st.Epochs[len(st.Epochs)-1]
	if phase == PhaseAudit {
		return newest.AfterAuditRound
	}
	return newest.AfterReviewRound
}

// EpochPosition returns where an epoch started now sits in phase: after the
// last recorded round that read an older text than specHash, so the rounds
// over the current text stay uncut. A round reads the current text only when
// its hash is of its own emission snapshot and equals specHash; an unmarked or
// no-emission round's reading cannot be established and counts as older. The
// position never moves backwards: the newest epoch's position is its floor,
// so a result equal to EpochCut means the epoch would cut nothing new.
//
// A spec reverted to an earlier text cuts the rounds that read that text the
// first time as well, since the position is one number per phase; the result
// errs toward more review, as everything an epoch cuts does.
func EpochPosition(st *ReviewState, phase, specHash string) int {
	pos := EpochCut(st, phase)
	if st == nil {
		return pos
	}
	rounds := st.ReviewRounds
	if phase == PhaseAudit {
		rounds = st.AuditRounds
	}
	for i := len(rounds) - 1; i >= 0; i-- {
		if rounds[i].HashScheme != HashSchemeSnapshot || rounds[i].SpecHash != specHash {
			return max(pos, rounds[i].Round)
		}
	}
	return pos
}

// RoundsSinceEpoch returns the recorded rounds of phase after the newest epoch,
// in recorded order: the rounds whose findings an emission may still carry.
// Every recorded round when there is no epoch.
func RoundsSinceEpoch(st *ReviewState, phase string) []ReviewRound {
	if st == nil {
		return nil
	}
	rounds := st.ReviewRounds
	if phase == PhaseAudit {
		rounds = st.AuditRounds
	}
	cut := EpochCut(st, phase)
	for i := range rounds {
		if rounds[i].Round > cut {
			return rounds[i:]
		}
	}
	return rounds[len(rounds):]
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// enforceImportConvergence blocks an import whose spec has recorded review
// rounds and whose review loop has not ended. It reads engine.ReviewLoopDone,
// the verdict --status --check, next_action and tp resume read: converged, or
// the round cap reached with every finding of the latest round dispositioned.
// The spec path is pinned to the import target's directory, matching how
// workflow resolution reads the spec field after the write.
func enforceImportConvergence(targetPath string, tf *model.TaskFile) *engine.LoopDone {
	stateSpec := filepath.Join(filepath.Dir(targetPath), filepath.Base(tf.Spec))

	st, err := engine.LoadReviewState(stateSpec)
	if err != nil {
		// A directory holding only snapshots recorded nothing, so it reads as
		// "no recorded rounds" here rather than as corruption — the same window
		// every other state reader now accepts. Lost history still aborts.
		if !engine.IsRebuildableStateIndex(err) {
			exitStateError(err)
			return nil
		}
		st = nil
	}
	if st == nil || len(st.ReviewRounds) == 0 {
		output.Info("review convergence not verified (no recorded rounds)")
		return nil
	}

	// Enforcement uses the resolved (project-layered) values, so a thinned task
	// file inherits the project requirement rather than reading the raw
	// task-file block alone.
	wfResolved, _ := engine.ResolveWorkflow(stateSpec, flagFile)
	specHash, hashErr := engine.SpecHash(stateSpec)
	if hashErr != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %s", stateSpec), hashErr.Error())
		os.Exit(ExitFile)
		return nil
	}
	done := engine.ReviewLoopDone(stateSpec, st.ReviewRounds, wfResolved.ReviewCleanRounds, wfResolved.ReviewMaxRounds, specHash, wfResolved.ReviewConvergeOn)
	lastRound := st.ReviewRounds[len(st.ReviewRounds)-1].Round

	if settledByLoopVerdict(done, wfResolved.ReviewMaxRounds, lastRound) {
		return &done
	}

	required := wfResolved.ReviewCleanRounds
	hint := "record the remaining clean rounds with tp review --record, or import with user-approved --force"
	// Review-convergence enforcement uses the live severity-aware predicate so a
	// blocking-policy round whose only survivors are medium/low counts clean,
	// consistent with tp review --status/--record.
	consecutive := engine.ReviewConsecutiveClean(stateSpec, st.ReviewRounds, wfResolved.ReviewConvergeOn)
	if consecutive < required {
		output.Error(ExitValidation, fmt.Sprintf("review not converged: %d consecutive clean rounds, %d required", consecutive, required), hint)
		os.Exit(ExitValidation)
		return nil
	}
	if engine.StateStale(st.ReviewRounds, specHash) {
		output.Error(ExitValidation, fmt.Sprintf("spec changed since round %d was recorded", lastRound), hint)
		os.Exit(ExitValidation)
		return nil
	}
	return &done
}

// settledByLoopVerdict handles the states the loop verdict decides on its own
// and reports whether it did: a converged loop imports; a loop the cap ended
// imports with a note naming what the cap waived, since no round verified it;
// a loop at the cap with a finding still open is refused with the way out.
// Anything else falls through to the clean-rounds and staleness checks.
// The verdict is returned to the caller for the import payload.
func settledByLoopVerdict(done engine.LoopDone, maxRounds, lastRound int) bool {
	switch {
	case done.Done && done.By == engine.DoneByCap:
		note := fmt.Sprintf("importing at the review round cap: every finding of round %d carries a disposition", lastRound)
		if done.FixedAtCap > 0 {
			note += fmt.Sprintf("; %d dispositioned fixed, which no round re-read", done.FixedAtCap)
		}
		if done.StaleWaived {
			note += fmt.Sprintf("; spec changed since round %d, not reviewed", lastRound)
		}
		output.Notice(note)
		return true
	case done.Done:
		return true
	case done.CapReached:
		msg := fmt.Sprintf("review reached its %d-round cap with %d finding(s) of round %d carrying no disposition", maxRounds, done.Undisposed, lastRound)
		if done.BlockingFixedAtCap > 0 {
			msg += fmt.Sprintf(" and %d blocking finding(s) marked fixed that no round re-read: the operator accepts them with evidence or raises the cap by one for a verification round", done.BlockingFixedAtCap)
		}
		output.Error(ExitValidation, msg, budgetEscalationHint)
		os.Exit(ExitValidation)
		return true
	}
	return false
}

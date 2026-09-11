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
// workflow resolution reads the spec field after the write. Where the loop
// lets the import go ahead, the registered checks decide last, as they do for
// --status --check (refuseImportOnChecks).
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
	wfResolved, checksTaskFile := engine.ResolveWorkflow(stateSpec, flagFile)
	specHash, hashErr := engine.SpecHash(stateSpec)
	if hashErr != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %s", stateSpec), hashErr.Error())
		os.Exit(ExitFile)
		return nil
	}
	done := engine.ReviewLoopDone(stateSpec, st.ReviewRounds, wfResolved.ReviewCleanRounds, wfResolved.ReviewMaxRounds, specHash, wfResolved.ReviewConvergeOn)
	lastRound := st.ReviewRounds[len(st.ReviewRounds)-1].Round

	if settledByLoopVerdict(done, wfResolved.ReviewMaxRounds, lastRound) {
		refuseImportOnChecks(stateSpec, &wfResolved, checksTaskFile)
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
	refuseImportOnChecks(stateSpec, &wfResolved, checksTaskFile)
	return &done
}

// refuseImportOnChecks runs the registered checks at the moment the loop lets
// the import go ahead — the done moment --record runs them at — and refuses
// the import when one failed or could not run, as `tp review --status --check`
// exits 1 on it: exit 1, naming the first such check and what it did. With no
// check registered it runs nothing, and with every check passing it returns.
// --force never reaches it: the caller skips every convergence check.
func refuseImportOnChecks(specPath string, wf *model.Workflow, taskFilePath string) {
	if len(wf.Checks) == 0 {
		return
	}
	results, _ := runMechanicalChecks(wf, taskFilePath)
	failing := checkVerdict(wf.Checks, results, taskFilePath).Failing
	if len(failing) == 0 {
		return
	}
	msg := "review checks do not pass: " + failing[0].FixClause()
	if more := len(failing) - 1; more > 0 {
		msg += fmt.Sprintf(" (and %d more)", more)
	}
	hint := "tp review " + specPath + " --status --check shows each check's output and exits 0 once every registered check passes; or import with user-approved --force"
	output.Error(ExitValidation, msg, hint)
	os.Exit(ExitValidation)
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

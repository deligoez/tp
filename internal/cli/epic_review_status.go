package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicStatusResult is tp epic review --status's payload (spec/1.3.0.md §5).
type epicStatusResult struct {
	StateDir         string                       `json:"state_dir"`
	Round            int                          `json:"round"`
	ConsecutiveClean int                          `json:"consecutive_clean"`
	Converged        bool                         `json:"converged"`
	Done             bool                         `json:"done"`
	DoneBy           *string                      `json:"done_by"`
	Stale            bool                         `json:"stale"`
	InFlightRound    *int                         `json:"in_flight_round"`
	MechanicalChecks []engine.EpicMechanicalCheck `json:"mechanical_checks"`
	NextAction       string                       `json:"next_action"`
}

// epicReviewPolicy resolves review_clean_rounds, review_converge_on and
// review_max_rounds from the project layer over the built-in default; a
// malformed project config exits 3.
func epicReviewPolicy(tpDir string) engine.EpicReviewPolicy {
	pc, _, err := engine.LoadProjectConfig(tpDir)
	if err != nil {
		output.Error(ExitFile, err.Error(), "fix .tp/config.json")
		os.Exit(ExitFile)
	}
	wf := engine.ResolveWorkflowLayers(&model.WorkflowOverride{}, &pc.Workflow)
	return engine.EpicReviewPolicy{CleanRounds: wf.ReviewCleanRounds, MaxRounds: wf.ReviewMaxRounds, ConvergeOn: wf.ReviewConvergeOn}
}

// epicReviewLoop reads the slice's loop verdict, exiting 3 on unreadable state.
func epicReviewLoop(run *epicReviewRun) engine.EpicReviewStatus {
	st, err := engine.EpicReviewLoopStatus(run.stateDir, run.all, epicReviewPolicy(run.tpDir))
	if err != nil {
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	return st
}

// refuseEpicReviewAtCap exits 4 with tp review's escalation hint when the
// slice's recorded rounds reached review_max_rounds and it is not converged:
// a reached cap refuses emission and record alike.
func refuseEpicReviewAtCap(run *epicReviewRun) {
	st := epicReviewLoop(run)
	if !st.CapReached {
		return
	}
	output.Error(ExitState, fmt.Sprintf("epic review round budget exhausted: %d rounds recorded for %s and not converged",
		st.Round, epicSliceLabel(run.key)), budgetEscalationHint)
	os.Exit(ExitState)
}

// runEpicReviewStatus prints the slice's loop status. With check it exits 1
// unless the loop is done and every mechanical check passes.
func runEpicReviewStatus(key engine.EpicSliceKey, check bool) {
	run := loadEpicReviewRun(key)
	st := epicReviewLoop(&run)
	checksPass := epicChecksPass(run.checks)
	result := epicStatusResult{
		StateDir:         epicReviewStateDirOut(&run),
		Round:            st.Round,
		ConsecutiveClean: st.ConsecutiveClean,
		Converged:        st.Converged,
		Done:             st.Done,
		Stale:            st.Stale,
		MechanicalChecks: run.checks,
		NextAction:       epicStatusNextAction(key, &st, checksPass),
	}
	if st.Done {
		result.DoneBy = &st.DoneBy
	}
	if st.InFlightRound > 0 {
		result.InFlightRound = &st.InFlightRound
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	if check && (!st.Done || !checksPass) {
		os.Exit(ExitValidation)
	}
}

// epicChecksPass reports whether every mechanical check passed.
func epicChecksPass(checks []engine.EpicMechanicalCheck) bool {
	for _, c := range checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// epicStatusNextAction names the one step the slice's loop waits on.
func epicStatusNextAction(key engine.EpicSliceKey, st *engine.EpicReviewStatus, checksPass bool) string {
	label := epicSliceLabel(key)
	switch {
	case st.Done && !checksPass:
		return "the slice's loop is done but a mechanical check fails: fix the errors tp epic lint reports"
	case st.Done:
		return "the slice's loop is done by " + st.DoneBy + ": nothing to run"
	case st.CapReached && st.Stale:
		return "the graph changed after the round cap: raising review_max_rounds for a verification round is the operator's decision"
	case st.CapReached:
		return "the round cap is reached: " + budgetEscalationHint
	case st.InFlightRound > 0:
		return fmt.Sprintf("record round %d: tp epic review %s --record <file>", st.InFlightRound, label)
	}
	return fmt.Sprintf("emit round %d: tp epic review %s", st.Round+1, label)
}

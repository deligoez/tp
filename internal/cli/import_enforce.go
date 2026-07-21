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
// rounds but is not converged or is stale. The spec path is pinned to the
// import target's directory, matching how workflow resolution reads the spec
// field after the write. The round budget never relaxes enforcement — an
// unconverged spec stays blocked regardless of budget.
func enforceImportConvergence(targetPath string, tf *model.TaskFile) {
	stateSpec := filepath.Join(filepath.Dir(targetPath), filepath.Base(tf.Spec))

	st, err := engine.LoadReviewState(stateSpec)
	if err != nil {
		exitStateError(err)
		return
	}
	if st == nil || len(st.ReviewRounds) == 0 {
		output.Info("review convergence not verified (no recorded rounds)")
		return
	}

	// Enforcement uses the resolved (project-layered) clean-rounds value, so a
	// thinned task file inherits the project requirement rather than reading the
	// raw task-file block alone.
	wfResolved, _ := engine.ResolveWorkflow(stateSpec, flagFile)
	required := wfResolved.ReviewCleanRounds

	hint := "record the remaining clean rounds with tp review --record, or import with user-approved --force"
	if wfResolved.ReviewMaxRounds > 0 && len(st.ReviewRounds) >= wfResolved.ReviewMaxRounds {
		hint = budgetEscalationHint
	}

	consecutive := engine.ConsecutiveClean(st.ReviewRounds)
	if consecutive < required {
		output.Error(ExitValidation, fmt.Sprintf("review not converged: %d consecutive clean rounds, %d required", consecutive, required), hint)
		os.Exit(ExitValidation)
		return
	}

	if specHash, hashErr := engine.SpecHash(stateSpec); hashErr == nil && engine.StateStale(st.ReviewRounds, specHash) {
		lastRound := st.ReviewRounds[len(st.ReviewRounds)-1].Round
		output.Error(ExitValidation, fmt.Sprintf("spec changed since round %d was recorded", lastRound), hint)
		os.Exit(ExitValidation)
		return
	}
}

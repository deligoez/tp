package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// budgetEscalationHint names the ways forward when a round budget is
// exhausted: uncounted passes first, then user-approved escapes only.
const budgetEscalationHint = "exhaust the tail with uncounted passes (delta pass, class sweep); then, with user approval, raise the cap via tp set --workflow review_max_rounds/audit_max_rounds for the confirming full-panel rounds, or import with user-approved --force"

// refuseIfBudgetExhausted exits 4 when a round cap is set, recorded rounds
// have reached it, and the sequence is not converged. Evaluated before line
// parsing, row validation, and any state write.
func refuseIfBudgetExhausted(kind, specPath string, rounds []engine.ReviewRound, capRounds, requiredClean int) {
	if capRounds <= 0 || len(rounds) < capRounds {
		return
	}
	if specHash, err := engine.SpecHash(specPath); err == nil && engine.Converged(rounds, requiredClean, specHash) {
		return
	}
	output.Error(ExitState, fmt.Sprintf("%s round budget exhausted: %d rounds recorded with a cap of %d and not converged", kind, len(rounds), capRounds), budgetEscalationHint)
	os.Exit(ExitState)
}

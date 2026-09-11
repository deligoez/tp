package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// budgetEscalationHint names the way out when a round cap is reached: the
// cap admits no further round, so the remaining findings of the latest round
// are dispositioned in its recorded file, then the loop ends and import or
// release proceeds. Raising the cap stays a user-approved decision.
const budgetEscalationHint = "disposition each remaining finding in the latest recorded round file — tp review|audit <round file> --resolve <selector> fixed|wontfix|duplicate \"<evidence>\" (accepting a blocking finding is the operator's decision); raising review_max_rounds/audit_max_rounds or import --force stay user-approved"

// refuseIfBudgetExhausted exits 4 when a round cap is set, recorded rounds
// have reached it, and the sequence is not converged. Evaluated before line
// parsing, row validation, and any state write. The review caller passes its
// effective review_converge_on so the refusal uses the live severity-aware
// predicate (ReviewConverged) and agrees with tp review --status/--record;
// audit callers pass "" and keep the frozen-flag Converged.
func refuseIfBudgetExhausted(kind, specPath string, rounds []engine.ReviewRound, capRounds, requiredClean int, convergeOn string) {
	if capRounds <= 0 || len(rounds) < capRounds {
		return
	}
	if specHash, err := engine.SpecHash(specPath); err == nil {
		converged := engine.Converged(rounds, requiredClean, specHash)
		if kind == "review" {
			converged = engine.ReviewConverged(specPath, rounds, requiredClean, specHash, convergeOn)
		}
		if converged {
			return
		}
	}
	output.Error(ExitState, fmt.Sprintf("%s round budget exhausted: %d rounds recorded with a cap of %d and not converged", kind, len(rounds), capRounds), budgetEscalationHint)
	os.Exit(ExitState)
}

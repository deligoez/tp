package cli

import (
	"fmt"
	"os"
	"strings"

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
// audit callers pass "" and keep the frozen-flag Converged. st is the spec's
// state, nil when none; the review refusal's hint leads with the carried
// blocking rows, which may sit in a round before the latest.
func refuseIfBudgetExhausted(kind, specPath string, st *engine.ReviewState, capRounds, requiredClean int, convergeOn string) {
	rounds := []engine.ReviewRound{}
	if st != nil {
		rounds = st.AuditRounds
		if kind == "review" {
			rounds = st.ReviewRounds
		}
	}
	if capRounds <= 0 || len(rounds) < capRounds {
		return
	}
	hint := budgetEscalationHint
	if specHash, err := engine.SpecHash(specPath); err == nil {
		converged := engine.Converged(rounds, requiredClean, specHash)
		if kind == "review" {
			converged = engine.ReviewConverged(specPath, st, requiredClean, specHash, convergeOn)
		}
		if converged {
			return
		}
	}
	if kind == "review" {
		// The carried clause names each row's own way out, so it replaces the
		// latest-round hint rather than contradicting it; only the cap's tail
		// stays, unless the clause already names raising it.
		if carried := engine.ReviewCarriedBlocking(specPath, st, convergeOn); len(carried) > 0 {
			hint = engine.CarriedBlockingClause(carried, specPath, true)
			if !strings.Contains(hint, engine.EscalateRaiseReviewCap) {
				hint += "; raising review_max_rounds stays user-approved (" + engine.EscalateRaiseReviewCap + ")"
			}
		}
	}
	output.Error(ExitState, fmt.Sprintf("%s round budget exhausted: %d rounds recorded with a cap of %d and not converged", kind, len(rounds), capRounds), hint)
	os.Exit(ExitState)
}

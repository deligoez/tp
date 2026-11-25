package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

func resolvedAs(status string) map[string]any {
	return map[string]any{"resolved": map[string]any{"status": status}}
}

// With no fixed finding the next step is the loop's own state, read from
// --status; a wontfix or duplicate never asks for a round that reads a repair.
func TestReviewResolvedNextStep_NoFixedFindingAsksForStatus(t *testing.T) {
	step := reviewResolvedNextStep([]map[string]any{resolvedAs("wontfix"), resolvedAs("duplicate")})
	assert.Equal(t, "tp review <spec> --status, whose next_action names the next step", step)
}

// A fixed finding outside a run names the next round's two commands; inside a
// run (TP_ROUND set) the unit stops, since the driver emits that round.
func TestReviewResolvedNextStep_FixedNamesTheRoundOrStopsInARun(t *testing.T) {
	findings := []map[string]any{resolvedAs("wontfix"), resolvedAs("fixed")}
	t.Setenv(engine.EnvRound, "")
	assert.Contains(t, reviewResolvedNextStep(findings), "tp review <spec>, then tp review <spec> --record-inbox")
	t.Setenv(engine.EnvRound, "2")
	assert.Equal(t, "stop: the run emits the next review round, which verifies the repair", reviewResolvedNextStep(findings))
}

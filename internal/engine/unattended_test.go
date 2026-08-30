package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// §5.1 / test 64: TP_UNATTENDED activates on any present, non-empty value other
// than "0", and does not activate on unset, empty or "0". The vocabulary is
// deliberately permissive on the active side — a driver that writes "1" and a
// harness that writes "true" must both fence the unit.
func TestUnattendedActive_ActivationVocabulary(t *testing.T) {
	for _, value := range []string{"1", "true", "yes", "00", "0.0", " "} {
		assert.True(t, UnattendedActive(value), "%q activates unattended mode", value)
	}
	for _, value := range []string{"", "0"} {
		assert.False(t, UnattendedActive(value), "%q leaves unattended mode off", value)
	}
}

// Unattended reads the same variable the driver exports (§3.1.1), so an unset
// variable — the ordinary interactive case — is never unattended.
func TestUnattended_ReadsTheChildEnvironmentVariable(t *testing.T) {
	t.Setenv(EnvUnattended, "")
	assert.False(t, Unattended(), "an empty TP_UNATTENDED is not unattended")

	t.Setenv(EnvUnattended, "0")
	assert.False(t, Unattended(), "TP_UNATTENDED=0 is not unattended")

	t.Setenv(EnvUnattended, "1")
	assert.True(t, Unattended(), "TP_UNATTENDED=1 is unattended")
}

// §5.1: the comparison is against the currently resolved value, an equal or
// lower value is accepted, and 0 is *disabled* rather than *lowest* for the
// fields whose 0 means no cap at all.
func TestUnattendedRaise_ComparesAgainstTheResolvedValue(t *testing.T) {
	assert.True(t, UnattendedRaise("review_max_rounds", 6, 5), "6 over a resolved 5 is a raise")
	assert.False(t, UnattendedRaise("review_max_rounds", 5, 5), "an equal value is accepted")
	assert.False(t, UnattendedRaise("review_max_rounds", 4, 5), "a lower value is accepted")

	assert.True(t, UnattendedRaise("review_max_rounds", 0, 5), "0 disables the cap, so it is a raise")
	assert.False(t, UnattendedRaise("review_max_rounds", 0, 0), "0 where 0 already resolves changes nothing")
	assert.False(t, UnattendedRaise("review_max_rounds", 7, 0), "a real cap under an uncapped resolve is a lowering")

	assert.True(t, UnattendedRaise("run_max_budget_usd", 0, 5), "a budget of 0 is disabled, so it is a raise")
	assert.False(t, UnattendedRaise("run_max_budget_usd", 2.5, 5), "a smaller budget is accepted")
	assert.True(t, UnattendedRaise("run_max_unit_budget_usd", 0, 1), "the per-unit budget's 0 is disabled too")

	// run_max_unit_retries' 0 is the fewest attempts, not "disabled".
	assert.False(t, UnattendedRaise("run_max_unit_retries", 0, 3), "0 retries is the strictest value")
	assert.True(t, UnattendedRaise("run_max_unit_retries", 3, 0), "more retries than resolved is a raise")
}

// The fenced sets are what §5.1's table names: the two round budgets and the
// five driver caps by value, runner and notify_cmd absolutely.
func TestFencedFields_CoverEveryFieldTheFenceNames(t *testing.T) {
	for _, field := range []string{
		"review_max_rounds", "audit_max_rounds",
		"run_max_units", "run_max_wall_clock_seconds",
		"run_max_budget_usd", "run_max_unit_budget_usd", "run_max_unit_retries",
	} {
		assert.True(t, FencedCapField(field), "%s is fenced by value", field)
		assert.False(t, FencedCommandField(field), "%s is not a driver command", field)
	}
	assert.False(t, FencedCapField("review_clean_rounds"), "a convergence count is not a budget")
	assert.False(t, FencedCapField("quality_gate"))

	assert.True(t, FencedCommandField("runner"))
	assert.True(t, FencedCommandField("notify_cmd"))
	assert.False(t, FencedCommandField("quality_gate"))
}

// ResolvedCapValue reads the fenced field out of an already-resolved workflow,
// which is what makes the comparison layer-agnostic: the fence never re-reads a
// layer of its own, so the environment cannot supply the number it compares to.
func TestResolvedCapValue_ReadsEveryFencedField(t *testing.T) {
	wf := &model.Workflow{
		ReviewMaxRounds:        3,
		AuditMaxRounds:         4,
		RunMaxUnits:            50,
		RunMaxWallClockSeconds: 600,
		RunMaxBudgetUSD:        12.5,
		RunMaxUnitBudgetUSD:    1.25,
		RunMaxUnitRetries:      2,
	}
	for field, want := range map[string]float64{
		"review_max_rounds":          3,
		"audit_max_rounds":           4,
		"run_max_units":              50,
		"run_max_wall_clock_seconds": 600,
		"run_max_budget_usd":         12.5,
		"run_max_unit_budget_usd":    1.25,
		"run_max_unit_retries":       2,
	} {
		got, ok := ResolvedCapValue(wf, field)
		assert.True(t, ok, "%s is a fenced cap field", field)
		assert.InDelta(t, want, got, 1e-9, "%s resolves to its workflow value", field)
	}

	_, ok := ResolvedCapValue(wf, "quality_gate")
	assert.False(t, ok, "a non-cap field has no resolved cap value")
}

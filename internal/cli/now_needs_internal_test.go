package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// Every needs_you text tp now can write is a plain sentence: no tp command
// and no flag, for every blocker code resume emits (with and without an
// operator decision in its data), every decision a unit can escalate, and
// every way a run can stop waiting for a person.
// The blocker messages themselves name commands, which is why the texts are
// written per code rather than copied.
func TestNowNeedTextsCarryNoCommand(t *testing.T) {
	t.Parallel()
	codes := []string{"no-ready-task", "epic-blocked", "review-carried-blocking",
		"review-budget-exhausted", "audit-budget-exhausted", "spec-stale", "a-future-code"}
	var needs []nowNeed
	for _, code := range codes {
		for _, data := range []map[string]any{
			{},
			{"epic": "payments", "held": []any{}, "operator": map[string]any{"decision": "x", "commands": []string{"tp review spec.md --force"}}},
		} {
			needs = append(needs, blockerNeed(&engine.Blocker{Code: code, Class: engine.ClassEscalate,
				Message: "run tp review spec.md --record-inbox", Data: data}))
		}
	}
	for _, decision := range engine.EscalationDecisions() {
		needs = append(needs, escalationNeed(&engine.Escalation{Decision: decision, UnitID: "u",
			Evidence: "tp set --workflow audit_max_rounds=5"}))
	}
	stopped := &engine.RunState{Units: []engine.RunUnitRow{{ID: "u"}}}
	needs = append(needs, runNeeds(stopped, runStateCrashed, nil)...)
	for _, reason := range []engine.StopReason{engine.StopCapUnits, engine.StopCapWallClock, engine.StopCapBudget,
		engine.StopEscalation, engine.StopUnitFailure, engine.StopInterrupted, engine.StopDriverError,
		engine.StopWorktreeSetupFailed, "a-future-reason"} {
		stopped.StopReason = &reason
		runNeed := runNeeds(stopped, runStateStopped, nil)
		assert.Len(t, runNeed, 1, "stop reason %s waits for a person", reason)
		needs = append(needs, runNeed...)
	}
	for _, n := range needs {
		assert.NotEmpty(t, n.Text, "every need has a text: %+v", n)
		assert.NotContains(t, n.Text, "tp ", "no tp command in %q", n.Text)
		assert.NotContains(t, n.Text, "--", "no flag in %q", n.Text)
		assert.Contains(t, []string{needBlocker, needEscalation, needDecision, needCap, needLand, needRun}, n.Kind)
	}
}

// A run need names its last unit only when that unit works a task: a review
// or audit unit's id is a round number, which read as a task (seen live: "1
// failed every attempt").
func TestNowRunNeedNamesOnlyATaskUnit(t *testing.T) {
	t.Parallel()
	failed := engine.StopUnitFailure
	audit := &engine.RunState{StopReason: &failed, Units: []engine.RunUnitRow{{Kind: engine.UnitAuditRecord, ID: "1"}}}
	assert.Empty(t, runNeeds(audit, runStateStopped, nil)[0].ID)
	task := &engine.RunState{StopReason: &failed, Units: []engine.RunUnitRow{{Kind: engine.UnitImplement, ID: "seed"}}}
	assert.Equal(t, "seed", runNeeds(task, runStateStopped, nil)[0].ID)
}

// Every needs_you entry carries a code that does not change with its text,
// so the pane can word it in its own language: a run's stop reason (or
// crashed), an escalation's decision name, a blocker's code (also when it
// names an operator decision), a cap's workflow field, and land-request.
func TestNowNeedsCarryAStableCode(t *testing.T) {
	t.Parallel()
	stopped := &engine.RunState{Units: []engine.RunUnitRow{{ID: "u"}}}
	assert.Equal(t, "crashed", runNeeds(stopped, runStateCrashed, nil)[0].Code)
	for _, reason := range []engine.StopReason{engine.StopUnitFailure, engine.StopCapBudget, "a-future-reason"} {
		stopped.StopReason = &reason
		assert.Equal(t, string(reason), runNeeds(stopped, runStateStopped, nil)[0].Code)
	}
	for _, decision := range engine.EscalationDecisions() {
		assert.Equal(t, decision, escalationNeed(&engine.Escalation{Decision: decision, UnitID: "u"}).Code)
	}
	for _, code := range []string{"no-ready-task", "epic-blocked", "review-carried-blocking", "spec-stale", "a-future-code"} {
		assert.Equal(t, code, blockerNeed(&engine.Blocker{Code: code}).Code)
		operator := map[string]any{"operator": map[string]any{"decision": "x"}}
		assert.Equal(t, code, blockerNeed(&engine.Blocker{Code: code, Data: operator}).Code)
	}
	assert.Equal(t, "review_max_rounds", blockerNeed(&engine.Blocker{Code: "review-budget-exhausted"}).Code)
	assert.Equal(t, "audit_max_rounds", blockerNeed(&engine.Blocker{Code: "audit-budget-exhausted"}).Code)
	assert.Equal(t, "land-request", landNeed("t1").Code)
}

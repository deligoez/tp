package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// Every needs_you text tp now can write is a plain sentence: no tp command
// and no flag, for every blocker code resume emits (with and without an
// operator decision in its data) and every decision a unit can escalate.
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
	for _, n := range needs {
		assert.NotEmpty(t, n.Text, "every need has a text: %+v", n)
		assert.NotContains(t, n.Text, "tp ", "no tp command in %q", n.Text)
		assert.NotContains(t, n.Text, "--", "no flag in %q", n.Text)
		assert.Contains(t, []string{needBlocker, needEscalation, needDecision, needCap, needLand}, n.Kind)
	}
}

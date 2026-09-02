package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// TestEscalationDecision_AuditConvergeOnHasItsOwnName covers v0.37.0 §7 row 14.
//
// The point of a closed --decision enum is that the stop is nameable: a run
// stopped under `other` is recoverable only from the free text of --evidence,
// which no driver can route on. audit_converge_on is §3's fenced field, so a
// unit that intends the relax must have a name for what it is asking, and the
// fence's hint is where it reads that name.
func TestEscalationDecision_AuditConvergeOnHasItsOwnName(t *testing.T) {
	assert.Equal(t, "audit-converge-on", escalationDecision("audit_converge_on"),
		"§3's field maps to its own decision rather than falling through to other")
}

// TestEscalationDecision_ReturnsOnlyDocumentedDecisions is the guard that keeps
// escalationDecision's literals from drifting out of engine's enum.
//
// The two live in different packages and are written as plain strings on both
// sides, so nothing but this test connects them. A field mapped to a name the
// enum does not carry is worse than one mapped to `other`: the hint tells the
// unit to run a `tp escalate` invocation tp itself refuses as a usage error,
// and the unit has no way to record anything at all.
//
// The fallthrough is asserted on a real fenced field rather than a made-up
// name, so the row cannot be satisfied by a switch that has stopped matching
// anything.
func TestEscalationDecision_ReturnsOnlyDocumentedDecisions(t *testing.T) {
	for _, field := range []string{
		"review_max_rounds",
		"audit_max_rounds",
		"audit_converge_on",
		"runner",
		"notify_cmd",
		"run_max_units",
		"run_max_wall_clock_seconds",
		"not_a_field",
		"",
	} {
		decision := escalationDecision(field)
		assert.True(t, engine.IsEscalationDecision(decision),
			"%q maps to %q, which tp escalate would refuse as a usage error", field, decision)
	}

	assert.Equal(t, engine.EscalateOther, escalationDecision("run_max_units"),
		"a fenced field with no name of its own still escalates as other")
}

package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildNextAction_DecomposeGatesOnRegisteredChecks: the decompose action
// names the check gate when the resolved workflow registers a check, and is
// untouched when it registers none.
func TestBuildNextAction_DecomposeGatesOnRegisteredChecks(t *testing.T) {
	t.Parallel()
	tf := &model.TaskFile{Spec: "spec.md"}

	gated := BuildNextAction(PhaseDecompose, "spec.md", tf, nil, true)
	require.NotNil(t, gated.Command)
	assert.Equal(t, "tp review spec.md --status --check", *gated.Command)
	// §9.3: a phase carrying a tp command carries the command that briefs it.
	// The gate is read-only and idempotent, so it briefs itself.
	require.NotNil(t, gated.BriefCommand, "a gated step carries a brief_command")
	assert.Equal(t, *gated.Command, *gated.BriefCommand, "the same command in both fields")
	assert.Equal(t, CheckGateClause("spec.md")+"decompose the converged spec into tasks and tp import",
		gated.Summary, "the gate reads as it does in tp review --status")

	plain := BuildNextAction(PhaseDecompose, "spec.md", tf, nil, false)
	assert.Nil(t, plain.Command, "with no registered check, decompose names no tp command")
	assert.Nil(t, plain.BriefCommand)
	assert.Equal(t, "decompose the converged spec into tasks and tp import", plain.Summary)
}

// TestRenderNextAction_AnEscalateBlockerOutranksTheCheckGate: the gate is a
// step like any other, so an escalate blocker still empties next_action — the
// operator answers before anything runs.
func TestRenderNextAction_AnEscalateBlockerOutranksTheCheckGate(t *testing.T) {
	t.Parallel()
	gated := BuildNextAction(PhaseDecompose, "spec.md", &model.TaskFile{Spec: "spec.md"}, nil, true)
	require.NotNil(t, gated.Command, "the state under test: a decompose action carrying the gate")

	deferred := renderNextAction(gated, nil, []Blocker{
		{Code: "spec-stale", Class: ClassEscalate, Message: "the spec changed"},
	})
	assert.Nil(t, deferred.Command, "nothing runs until the operator answers")
	assert.Nil(t, deferred.BriefCommand)
	assert.Equal(t, "the spec changed", deferred.Summary)
}

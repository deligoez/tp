package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicReview_GateScopeJudgesTheScopeUnitAlone: the built-in gate-scope
// role runs fourth, is asked only scope:<m>, reads the live scope and the
// purpose header; phase-economist is asked the epics with the purpose and
// no gate.
func TestEpicReview_GateScopeJudgesTheScopeUnitAlone(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")

	out := runEpicReview(t, dir, "--milestone", "A")
	require.Equal(t, []string{"dod-satisfiability", "why-auditor", "phase-economist", "gate-scope"}, promptRoles(&out))
	scope := out.Prompts[3].Prompt
	assert.Contains(t, scope, "Asked units:\n---\nscope: A\n")
	assert.NotContains(t, scope, "\n---\nepic: a\n")
	assert.Contains(t, scope, "\nMilestone purposes:\n- A: prove the chain end to end\n")
	assert.Contains(t, scope, "Scope of gate A")
	assert.Contains(t, scope, "`scope:<milestone>`")
	economist := out.Prompts[2].Prompt
	assert.Contains(t, economist, "\nMilestone purposes:\n- A: prove the chain end to end\n")
	assert.NotContains(t, economist, "gate: A\n", "phase-economist no longer takes the gate")
	for _, p := range out.Prompts[:2] {
		assert.NotContains(t, p.Prompt, "Milestone purposes:", "%s does not get the purpose", p.Role)
		assert.NotContains(t, p.Prompt, "scope: A\n", "%s is not asked the scope unit", p.Role)
	}
}

// TestEpicReview_AnEpicSliceConvergesWithGateScopeSkipped: an epic slice holds
// no scope unit, so gate-scope sits in skipped_roles with no-units and the
// slice still converges.
func TestEpicReview_AnEpicSliceConvergesWithGateScopeSkipped(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	empty := filepath.Join(t.TempDir(), "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	assert.Equal(t, "no-units", epicSkipped(t, dir, "--epic", "b")["gate-scope"], "round 1 asks b, which gate-scope takes no kind of")
	for range 2 {
		runEpicReview(t, dir, "--epic", "b")
		_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--record", empty)
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}
	_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--status", "--check")
	assert.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestEpicReview_AConvergedSliceAtItsCapGainingAScopeUnitIsGrantable: a slice
// that converged at exactly review_max_rounds and then gains a stale scope
// unit refuses emission and is grantable; the cap is not lifted for it.
func TestEpicReview_AConvergedSliceAtItsCapGainingAScopeUnitIsGrantable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "review_max_rounds=2")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	recordEpicRows(t, dir, "")
	recordEpicRows(t, dir, "")
	_, _, code = runTP(t, dir, "epic", "review", "--milestone", "A", "--status", "--check")
	require.Equal(t, 0, code, "converged at round 2 of 2")

	setGateField(t, dir, "gates.A.threshold=ten users")
	_, stderr, code = runTP(t, dir, "epic", "review", "--milestone", "A")
	assert.Equal(t, 4, code, "emission is refused at the cap: %s", stderr)
	assert.Contains(t, stderr, "--grant-round")
	stdout, _, _ := runTP(t, dir, "epic", "review", "--milestone", "A", "--status")
	var st struct {
		Grantable  bool     `json:"grantable"`
		StaleUnits []string `json:"stale_units"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &st), stdout)
	assert.True(t, st.Grantable)
	assert.Contains(t, st.StaleUnits, "scope:A")
}

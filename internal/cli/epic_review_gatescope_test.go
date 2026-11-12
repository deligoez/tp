package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
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
	assert.Contains(t, scope, "purpose: prove the chain end to end\n", "the scope unit's block carries the purpose")
	assert.NotContains(t, scope, "Milestone purposes:", "a scope-only role reads the purpose once, in its block")
	assert.Empty(t, epicSkipped(t, dir, "--milestone", "A"), "all four built-in roles run, gate-scope not skipped")
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

// ejectedThreeRoles is a corpus ejected before gate-scope shipped: the other
// three built-in roles, by file.
func ejectedThreeRoles(t *testing.T, dir string) {
	t.Helper()
	files, err := engine.DefaultEpicCorpusFiles()
	require.NoError(t, err)
	roles := map[string]string{}
	for _, f := range files {
		if f.ID != "gate-scope" {
			roles[f.ID] = string(f.Data)
		}
	}
	writeEpicRoles(t, dir, roles)
}

// TestEpicReview_AnUnrecordedRoundReemitsAgainstTheCurrentCorpus answers how
// a role that joins the corpus reaches the review: the round out is recorded
// first, then the next round is built from the current graph and corpus, not
// replayed, so the new role and the scope unit it reads are in it. A corpus
// change under a round still out is the re-emission guard's
// (spec/backlog/epic-reemit-guard.md).
func TestEpicReview_AnUnrecordedRoundReemitsAgainstTheCurrentCorpus(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	ejectedThreeRoles(t, dir)
	first := runEpicReview(t, dir, "--milestone", "A")
	assert.NotContains(t, first.Units, "scope:A", "no role reads a scope unit yet")
	writeEpicRound(t, dir, &first, "")

	files, err := engine.DefaultEpicCorpusFiles()
	require.NoError(t, err)
	writeEpicRoles(t, dir, map[string]string{"gate-scope": string(files[3].Data)})
	next := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, 2, next.Round)
	assert.Contains(t, promptRoles(&next), "gate-scope")
	assert.Contains(t, next.Units, "scope:A")
	only := runEpicReview(t, dir, "--milestone", "A", "--role", "gate-scope")
	assert.Equal(t, []string{"gate-scope"}, promptRoles(&only))
}

// TestEpicReview_ABuiltInRoleMissingFromAnEjectedCorpusSaysWhy: --role
// gate-scope over a corpus ejected before it shipped names the cause and the
// way forward instead of a bare unknown role.
func TestEpicReview_ABuiltInRoleMissingFromAnEjectedCorpusSaysWhy(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	ejectedThreeRoles(t, dir)
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--role", "gate-scope")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "built-in graph role gate-scope is not in this project's corpus")
	assert.Contains(t, stderr, "tp init --eject-roles --graph --force")
	_, stderr, _ = runTP(t, dir, "epic", "review", "--milestone", "A", "--role", "no-such-role")
	assert.Contains(t, stderr, "unknown role: no-such-role", "a name tp never shipped keeps the plain refusal")
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
		recordEpicEmpty(t, dir, "--epic", "b")
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

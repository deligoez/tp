package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dirtyRow = `{"severity":"low","category":"c","location":"L1","finding":"still broken","suggestion":"s"}` + "\n"

func setupBudgetProject(t *testing.T, field string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", field+"=2")
	require.Equal(t, 0, code)
	return dir
}

func TestRoundBudget_ReviewRefusals(t *testing.T) {
	dir := setupBudgetProject(t, "review_max_rounds")

	// Two dirty rounds exhaust the cap
	for i := 0; i < 2; i++ {
		_, stderr, code := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code, "round %d: %s", i+1, stderr)
	}

	// Prompt generation refuses with exit 4 and the escalation hint
	_, stderr, code := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "budget exhausted")
	assert.Contains(t, stderr, "raise the cap")

	// A further --record is refused before any write
	_, stderr, code = recordRound(t, dir, "")
	assert.Equal(t, 4, code, "beyond-cap record refused even when clean")
	assert.Contains(t, stderr, "budget exhausted")
	_, err := os.Stat(filepath.Join(dir, ".tp-review", "spec", "review-round-3.ndjson"))
	assert.True(t, os.IsNotExist(err), "no round file written on refusal")

	// --status reports budget_exhausted: true
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["budget_exhausted"])
}

func TestRoundBudget_ConvergedAtCapNotRefused(t *testing.T) {
	dir := setupBudgetProject(t, "review_max_rounds")

	// Two clean rounds: at the cap but converged (required_clean_rounds=2)
	for i := 0; i < 2; i++ {
		_, _, code := recordRound(t, dir, "")
		require.Equal(t, 0, code)
	}

	_, _, code := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 0, code, "converged sequence is never refused")

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["budget_exhausted"], "converged means not exhausted")
}

func TestRoundBudget_AuditRefusals(t *testing.T) {
	dir := setupBudgetProject(t, "audit_max_rounds")

	for i := 0; i < 2; i++ {
		_, stderr, code := auditRecord(t, dir, `{"id":"x","status":"FAIL"}`+"\n")
		require.Equal(t, 0, code, "round %d: %s", i+1, stderr)
	}

	// Audit prompt generation refuses
	_, stderr, code := runTP(t, dir, "audit", "spec.md")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "audit round budget exhausted")

	// Beyond-cap audit record refused
	_, stderr, code = auditRecord(t, dir, "")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "budget exhausted")

	// tp audit --status reports the field
	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["budget_exhausted"])
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNonGoals_EmitsPromptsNoRuntime: tp emits prompts and instructs the
// orchestrator to spawn sub-agents — it never executes agents (non-goal 1).
func TestNonGoals_EmitsPromptsNoRuntime(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))

	prompts, ok := out["prompts"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, prompts, "tp emits prompts")
	loop := out["review_loop"].(map[string]any)
	assert.Contains(t, loop["instruction"], "sub-agent", "the orchestrator spawns agents; tp only emits prompts")
}

// TestNonGoals_NoAutoTaper: the emitted panel size is a durable corpus decision;
// recording rounds never trims the diversity panel (non-goal 3).
func TestNonGoals_NoAutoTaper(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))

	diversityRoles := func() int {
		stdout, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		byRole := reviewPromptsByRole(t, stdout)
		n := 0
		for r := range byRole {
			if r != "regression" {
				n++
			}
		}
		return n
	}

	before := diversityRoles()
	_, _, code := runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	after := diversityRoles()

	assert.Equal(t, 3, before, "the software default panel is 3 reviewers")
	assert.Equal(t, before, after, "the panel is never auto-tapered across rounds")
}

// TestNonGoals_NoNewRoleFromFrontmatter: a frontmatter override never creates a
// new role — an unknown override id is ignored (non-goal 6).
func TestNonGoals_NoNewRoleFromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	spec := "---\ntp:\n  review_roles:\n    brand-new-role:\n      focus:\n        - \"q\"\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)
	assert.NotContains(t, byRole, "brand-new-role", "overrides only extend existing roles; new roles are files, not frontmatter")
}

// TestNonGoals_NoAuditClustering: audit --record counts every non-PASS row; audit
// findings are never clustered (non-goal 8).
func TestNonGoals_NoAuditClustering(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	out, stderr, code := auditRecord(t, dir,
		`{"id":"a","status":"FAIL","class":"c","location":"§1"}`+"\n"+
			`{"id":"b","status":"FAIL","class":"c","location":"§1"}`+"\n")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, float64(2), out["findings"], "two same-(location,class) FAIL rows stay two findings")
}

// TestNonGoals_NoNewConfigWorkflowField: v0.25.0 adds no .tp/config.json workflow
// field (non-goal 4).
func TestNonGoals_NoNewConfigWorkflowField(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	stdout, _, code := runTP(t, dir, "config")
	require.Equal(t, 0, code)
	for _, forbidden := range []string{"review_roles", "audit_roles", "roles_hash", "eject"} {
		assert.NotContains(t, stdout, forbidden, "no new config workflow field %q was introduced", forbidden)
	}
}

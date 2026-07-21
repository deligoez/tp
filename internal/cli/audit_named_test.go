package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupThreeRoleAudit(t *testing.T) (stdout string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# P\n## Conventions\n- convention marker line\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	var stderr string
	stdout, stderr, code = runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "audit failed: %s", stderr)
	return stdout
}

func TestGenerateAuditPrompts_ThreeRolesPresent(t *testing.T) {
	stdout := setupThreeRoleAudit(t)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	prompts := out["prompts"].([]any)
	require.Len(t, prompts, 3, "exactly 3 prompts when all roles have items")
	assert.Equal(t, "spec-coverage", prompts[0].(map[string]any)["role"])
	assert.Equal(t, "security", prompts[1].(map[string]any)["role"])
	assert.Equal(t, "maintainability-conventions", prompts[2].(map[string]any)["role"])
}

func TestGenerateAuditPrompts_StructuredItems(t *testing.T) {
	stdout := setupThreeRoleAudit(t)
	byRole := auditPromptsByRole(t, stdout)
	spec := byRole["spec-coverage"]
	text := spec["prompt"].(string)
	items := spec["checklist_items"].([]any)
	require.NotEmpty(t, items)
	for _, item := range items {
		m := item.(map[string]any)
		assert.Contains(t, text, `"item_id":"`+m["item_id"].(string)+`"`, "JSON-array checklist in body matches ChecklistItems")
	}
}

func TestGenerateAuditPrompts_NoCategoryField(t *testing.T) {
	stdout := setupThreeRoleAudit(t)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	for _, p := range out["prompts"].([]any) {
		_, hasCategory := p.(map[string]any)["category"]
		assert.False(t, hasCategory, "auditPrompt JSON output has no Category key")
	}
}

func TestGenerateAuditPrompts_SpecExcerptOnlyForSpecCoverage(t *testing.T) {
	stdout := setupThreeRoleAudit(t)
	byRole := auditPromptsByRole(t, stdout)
	assert.Contains(t, byRole["spec-coverage"]["prompt"].(string), "## Spec Excerpt")
	assert.NotContains(t, byRole["security"]["prompt"].(string), "## Spec Excerpt")
	assert.NotContains(t, byRole["maintainability-conventions"]["prompt"].(string), "## Spec Excerpt")
}

func TestGenerateAuditPrompts_CLAUDEmdOnlyForMaintainability(t *testing.T) {
	stdout := setupThreeRoleAudit(t)
	byRole := auditPromptsByRole(t, stdout)
	assert.Contains(t, byRole["maintainability-conventions"]["prompt"].(string), "convention marker line")
	assert.NotContains(t, byRole["spec-coverage"]["prompt"].(string), "convention marker line")
	assert.NotContains(t, byRole["security"]["prompt"].(string), "convention marker line")
}

// TestAudit_CorpusDrivenEmission: a user auditor corpus drives emission — the
// emitted roles and their Role Rules come from the corpus, not the removed
// hardcoded persona/rules set (§7.2).
func TestAudit_CorpusDrivenEmission(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "spec-coverage.json"),
		[]byte(`{"id":"spec-coverage","title":"Spec Coverage","instructions":"I","focus":["MY CUSTOM COVERAGE RULE"]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. Widgets\n| Name | Type |\n|------|------|\n| a | b |\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "audit failed: %s", stderr)
	byRole := auditPromptsByRole(t, stdout)

	// The populated .tp/auditors corpus replaces the default 3-role set.
	require.Contains(t, byRole, "spec-coverage")
	assert.NotContains(t, byRole, "security", "the custom corpus replaces the embedded default")
	assert.NotContains(t, byRole, "maintainability-conventions")

	// Role Rules come from the corpus focus, not the removed hardcoded map.
	prompt := byRole["spec-coverage"]["prompt"].(string)
	assert.Contains(t, prompt, "MY CUSTOM COVERAGE RULE")
	assert.NotContains(t, prompt, "State-dependent behaviors", "the hardcoded default rule is gone")
}

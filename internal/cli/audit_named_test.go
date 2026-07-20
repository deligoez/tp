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

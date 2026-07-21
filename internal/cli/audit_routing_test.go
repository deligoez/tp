package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const routingSpec = `# Spec
## Table
| Col | Desc |
|-----|------|
| a | first |
## Steps
1. do the thing
`

func auditPromptsByRole(t *testing.T, stdout string) map[string]map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	byRole := map[string]map[string]any{}
	for _, p := range out["prompts"].([]any) {
		m := p.(map[string]any)
		byRole[m["role"].(string)] = m
	}
	return byRole
}

// TestRouteChecklist_Disjoint: each spec-derived item appears in exactly one
// role bucket; file-level items only in security/maintainability.
func TestRouteChecklist_Disjoint(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	// A security-relevant and a plain file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go", "--affected-files", "plain.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	prompts := out["prompts"].([]any)
	require.Len(t, prompts, 3, "all three roles populated")

	// Fixed order
	assert.Equal(t, "spec-coverage", prompts[0].(map[string]any)["role"])
	assert.Equal(t, "security", prompts[1].(map[string]any)["role"])
	assert.Equal(t, "maintainability-conventions", prompts[2].(map[string]any)["role"])

	// Disjoint routing: spec items only in spec-coverage; file_check only in
	// the file-checklist roles
	byRole := auditPromptsByRole(t, stdout)
	for _, item := range byRole["spec-coverage"]["checklist_items"].([]any) {
		typ := item.(map[string]any)["type"].(string)
		assert.Contains(t, []string{"table_row", "list_item", "task_acceptance", "finding"}, typ)
	}
	secItems := byRole["security"]["checklist_items"].([]any)
	require.Len(t, secItems, 1, "only the auth-matching file")
	sec0 := secItems[0].(map[string]any)
	assert.Equal(t, "file_check", sec0["type"])
	assert.Equal(t, "file-sec-0", sec0["item_id"])
	assert.Equal(t, "auth_helper.go", sec0["section"])
	for _, item := range byRole["maintainability-conventions"]["checklist_items"].([]any) {
		m := item.(map[string]any)
		assert.Equal(t, "file_check", m["type"])
		assert.Contains(t, m["item_id"], "file-maint-")
	}
}

// TestGenerateAuditPrompts_EmptyRoleOmitted: a role with zero checklist
// items is absent from prompts.
func TestGenerateAuditPrompts_EmptyRoleOmitted(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "plain.go")
	require.Equal(t, 0, code)

	byRole := auditPromptsByRole(t, stdout)
	_, hasSecurity := byRole["security"]
	assert.False(t, hasSecurity, "security role with no matching files is omitted")
	assert.Contains(t, byRole, "spec-coverage")
	assert.Contains(t, byRole, "maintainability-conventions")
}

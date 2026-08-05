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

// TestRouteChecklist_Disjoint: spec-derived items appear only in the
// spec-coverage prompt; every other role's items are file_check items over the
// same shared code-file list.
func TestRouteChecklist_Disjoint(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	// A keyword-matching and a plain file; both reach every shared-arm role.
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

	// spec-coverage holds every spec-derived item and no file_check item.
	byRole := auditPromptsByRole(t, stdout)
	specIDs := map[string]bool{}
	for _, item := range byRole["spec-coverage"]["checklist_items"].([]any) {
		m := item.(map[string]any)
		assert.Contains(t, []string{"table_row", "list_item", "task_acceptance", "finding"}, m["type"].(string))
		specIDs[m["item_id"].(string)] = true
	}
	require.NotEmpty(t, specIDs, "the fixture spec yields spec-derived items")

	// Every other role: file_check items over the same shared code-file list,
	// and no spec-derived item leaks into them.
	var shared []string
	for role, p := range byRole {
		if role == "spec-coverage" {
			continue
		}
		sections := make([]string, 0)
		for _, item := range p["checklist_items"].([]any) {
			m := item.(map[string]any)
			assert.Equal(t, "file_check", m["type"], "role %s holds only file_check items", role)
			assert.False(t, specIDs[m["item_id"].(string)], "spec-derived items appear only in spec-coverage")
			assert.Contains(t, m["item_id"], "file-"+role+"-")
			assert.NotRegexp(t, `^file-`+role+`-\d+$`, m["item_id"], "id is slug-based not positional")
			sections = append(sections, m["section"].(string))
		}
		assert.Equal(t, []string{"auth_helper.go", "plain.go"}, sections, "role %s gets the whole shared list, not a keyword filter", role)
		if shared == nil {
			shared = sections
			continue
		}
		assert.Equal(t, shared, sections, "every shared-arm role gets the identical list")
	}
}

// TestGenerateAuditPrompts_EmptyRoleOmitted: a role with zero checklist
// items is absent from prompts.
// TestGenerateAuditPrompts_SharedArmReachesEveryRole: the shared code-file
// list has no relevance filter, so a single file matching no priority keyword
// still gives security one file_check item — no role is skipped (§7 item 1).
func TestGenerateAuditPrompts_SharedArmReachesEveryRole(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "plain.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := auditPromptsByRole(t, stdout)
	secItems, ok := byRole["security"]
	require.True(t, ok, "security is emitted from the shared code-file list")
	items := secItems["checklist_items"].([]any)
	require.Len(t, items, 1, "one file_check item, one per selected code file")
	item := items[0].(map[string]any)
	assert.Equal(t, "file_check", item["type"])
	assert.Equal(t, "plain.go", item["section"])

	assert.Contains(t, byRole, "spec-coverage")
	assert.Contains(t, byRole, "maintainability-conventions")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, []any{}, out["skipped_roles"], "every auditor role emits a prompt")
}

// TestAudit_ContentKeywordDoesNotPromote: with the HEAD content channel gone,
// ranking reads the path only, so a file whose content mentions auth but whose
// path matches no keyword keeps its alphabetical position.
func TestAudit_ContentKeywordDoesNotPromote(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_one.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "m_notes.go"), []byte("package main\n\n// auth is only mentioned in the content\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z_two.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md",
		"--affected-files", "z_two.go", "--affected-files", "m_notes.go", "--affected-files", "a_one.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := auditPromptsByRole(t, stdout)
	entries := byRole["maintainability-conventions"]["affected_files"].([]any)
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.(map[string]any)["path"].(string))
	}
	assert.Equal(t, []string{"a_one.go", "m_notes.go", "z_two.go"}, paths,
		"a content-only auth match is ranked alphabetically, not promoted")
}

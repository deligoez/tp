package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditPrompts_BodyOrderAndEmbedding(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	claudeMD := "# Project\n## Conventions\n- exit codes 0-4\n- flock on writes\n## Other\nignored\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(claudeMD), 0o600))
	// spec-adjacent task file so CLAUDE.md resolves next to it
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := auditPromptsByRole(t, stdout)

	// §3.1 body order in the spec-coverage prompt
	spec := byRole["spec-coverage"]["prompt"].(string)
	idxRole := strings.Index(spec, "## Role\n")
	idxRules := strings.Index(spec, "## Role Rules")
	idxExcerpt := strings.Index(spec, "## Spec Excerpt")
	idxChecklist := strings.Index(spec, "## Checklist")
	idxFiles := strings.Index(spec, "## Affected Files (max 20)")
	idxSchema := strings.Index(spec, "## Output Schema")
	for name, pair := range map[string][2]int{
		"role<rules":    {idxRole, idxRules},
		"rules<excerpt": {idxRules, idxExcerpt},
		"excerpt<list":  {idxExcerpt, idxChecklist},
		"list<files":    {idxChecklist, idxFiles},
		"files<schema":  {idxFiles, idxSchema},
	} {
		assert.Less(t, pair[0], pair[1], name)
		assert.GreaterOrEqual(t, pair[0], 0, name)
	}

	// §3.2: checklist embedded as a JSON array, identical to checklist_items
	arrayStart := strings.Index(spec, "[\n")
	require.Greater(t, arrayStart, 0)
	items := byRole["spec-coverage"]["checklist_items"].([]any)
	first := items[0].(map[string]any)
	assert.Contains(t, spec, `"item_id":"`+first["item_id"].(string)+`"`, "prompt body embeds items as JSON")
	assert.Contains(t, spec, `"expected_evidence":`, "embedded items carry expected_evidence")
	for _, item := range items {
		assert.NotEmpty(t, item.(map[string]any)["expected_evidence"], "expected_evidence is never empty")
	}

	// §3.4: spec excerpt only in spec-coverage; CLAUDE.md only in maintainability
	sec := byRole["security"]["prompt"].(string)
	maint := byRole["maintainability-conventions"]["prompt"].(string)
	assert.NotContains(t, sec, "## Spec Excerpt")
	assert.NotContains(t, maint, "## Spec Excerpt")
	assert.Contains(t, maint, "## Project Context")
	assert.Contains(t, maint, "exit codes 0-4", "Conventions section excerpt included")
	assert.NotContains(t, maint, "ignored", "excerpt stops at the next heading")
	assert.NotContains(t, sec, "## Project Context")
	assert.NotContains(t, spec, "## Project Context")
}

func TestAuditPrompts_DeterministicRegeneration(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"finding":"first issue"}`+"\n"+`{"finding":"second issue"}`+"\n"), 0o600))

	run := func() string {
		stdout, _, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "plain.go", "--findings", findings)
		require.Equal(t, 0, code)
		return stdout
	}
	first := run()
	second := run()
	assert.Equal(t, first, second, "regeneration without input changes yields identical output and ids")

	byRole := auditPromptsByRole(t, first)
	ids := make([]string, 0)
	for _, item := range byRole["spec-coverage"]["checklist_items"].([]any) {
		ids = append(ids, item.(map[string]any)["item_id"].(string))
	}
	assert.Contains(t, ids, "finding-0")
	assert.Contains(t, ids, "finding-1")
	assert.Equal(t, "verify the fix for: first issue",
		findItemEvidence(t, byRole["spec-coverage"], "finding-0"))
}

func findItemEvidence(t *testing.T, prompt map[string]any, itemID string) string {
	t.Helper()
	for _, item := range prompt["checklist_items"].([]any) {
		m := item.(map[string]any)
		if m["item_id"] == itemID {
			return m["expected_evidence"].(string)
		}
	}
	t.Fatalf("item %s not found", itemID)
	return ""
}

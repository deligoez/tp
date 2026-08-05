package cli_test

import (
	"fmt"
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
	idxFiles := strings.Index(spec, "## Affected Files (")
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

	// §3.4: spec excerpt only in spec-coverage; CLAUDE.md never in spec-coverage
	sec := byRole["security"]["prompt"].(string)
	maint := byRole["maintainability-conventions"]["prompt"].(string)
	assert.NotContains(t, sec, "## Spec Excerpt")
	assert.NotContains(t, maint, "## Spec Excerpt")
	assert.Contains(t, maint, "## Project Context")
	assert.Contains(t, maint, "exit codes 0-4", "Conventions section excerpt included")
	assert.NotContains(t, maint, "ignored", "excerpt stops at the next heading")
	assert.NotContains(t, spec, "## Project Context")
}

// dispositionParagraph is §2.2's block, verbatim.
const dispositionParagraph = `## Disposition
A file containing nothing in this role's domain is a PASS, not a PARTIAL. Record it as
PASS with evidence_file set to that path and evidence_lines set to the full range you
read (for example "1-120"), meaning: the whole file was inspected and nothing in this
role's domain appears in it. Reserve PARTIAL and FAIL for a defect you actually found.
`

// TestAuditPrompts_DispositionBlock: the §2.2 paragraph is rendered verbatim
// exactly once in every shared-arm prompt, sits between the checklist and the
// affected files, never reaches spec-coverage, and is not repeated into any
// item's expected_evidence (§7 item 8).
func TestAuditPrompts_DispositionBlock(t *testing.T) {
	stdout := setupThreeRoleAudit(t)
	byRole := auditPromptsByRole(t, stdout)
	require.NotEmpty(t, byRole)

	for role, p := range byRole {
		prompt := p["prompt"].(string)
		if role == "spec-coverage" {
			assert.NotContains(t, prompt, "## Disposition", "spec-coverage never carries the disposition block")
			continue
		}
		assert.Equal(t, 1, strings.Count(prompt, dispositionParagraph), "role %s carries the disposition paragraph verbatim exactly once", role)

		idxChecklist := strings.Index(prompt, "## Checklist")
		idxDisposition := strings.Index(prompt, "## Disposition")
		idxFiles := strings.Index(prompt, "## Affected Files (")
		require.GreaterOrEqual(t, idxChecklist, 0, role)
		require.GreaterOrEqual(t, idxFiles, 0, role)
		assert.Less(t, idxChecklist, idxDisposition, "disposition follows the checklist in %s", role)
		assert.Less(t, idxDisposition, idxFiles, "disposition precedes the affected files in %s", role)
	}

	// The rule is stated once per prompt, never repeated per checklist item.
	for role, p := range byRole {
		for _, item := range p["checklist_items"].([]any) {
			evidence := item.(map[string]any)["expected_evidence"].(string)
			assert.NotContains(t, evidence, "PASS", "expected_evidence in %s stays the short inspect form", role)
		}
	}
}

// TestAuditPrompts_AffectedFilesHeaderForms: §2.6's two header forms — the
// untruncated "max <cap>" naming the role's own cap, and the counted
// "<n> of <total>" once the cap has bitten, with the total taken after the
// drop rules removed binaries and test fixtures.
func TestAuditPrompts_AffectedFilesHeaderForms(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	args := []string{"audit", "spec.md"}
	// 14 eligible code files: over CodeFileCap (10), under AuditFileCap (20).
	for i := 1; i <= 14; i++ {
		name := fmt.Sprintf("f%02d.go", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
		args = append(args, "--affected-files", name)
	}
	// Dropped by the filter, so neither total counts them.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "logo.png"), []byte("\x89PNG\r\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "testdata"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "testdata", "fixture.go"), []byte("package main\n"), 0o600))
	args = append(args, "--affected-files", "logo.png", "--affected-files", filepath.Join("testdata", "fixture.go"))

	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := auditPromptsByRole(t, stdout)

	// Shared arm: 10 of 14 — the counted form, total asserted post-drop.
	sec := byRole["security"]
	assert.Len(t, sec["affected_files"].([]any), 10)
	assert.Contains(t, sec["prompt"].(string), "## Affected Files (10 of 14)\n")
	assert.NotContains(t, sec["prompt"].(string), "## Affected Files (max",
		"the counted form does not restate the cap")

	// spec-coverage: nothing truncated, so the untruncated form naming 20.
	specCov := byRole["spec-coverage"]
	assert.Len(t, specCov["affected_files"].([]any), 14)
	assert.Contains(t, specCov["prompt"].(string), "## Affected Files (max 20)\n")

	// Shared arm, nothing truncated: the untruncated form naming CodeFileCap.
	shortRun := setupThreeRoleAudit(t)
	shortByRole := auditPromptsByRole(t, shortRun)
	assert.Contains(t, shortByRole["security"]["prompt"].(string), "## Affected Files (max 10)\n")
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

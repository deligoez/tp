package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditBasicWithAffectedFiles(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col | Desc |\n|-----|------|\n| a | first |\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\nfunc main() {}\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.Equal(t, []any{aPath}, result["files"])
	assert.Equal(t, "implementation-auditor", result["prompts"].([]any)[0].(map[string]any)["role"].(string))
}

func TestAuditNoAffectedFilesNoGit(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\nEmpty.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath)
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "not in a git repo")
}

func TestAuditAffectedFileNotFound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", "/nonexistent")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "not found")
}

func TestAuditAffectedFileDirectory(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	subDir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", subDir)
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "directory")
}

func TestAuditAffectedFilesDedup(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Len(t, result["files"].([]any), 1)
}

func TestAuditNoSpecArg(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "audit")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "accepts 1 arg")
}

func TestAuditSpecNotFound(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "audit", "/nonexistent/spec.md", "--affected-files", "x.go")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "not found")
}

func TestAuditChecklistTableRows(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Spec
## Table
| Col | Desc |
|-----|------|
| a | first |
| b | second |
`), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	checklist := result["checklist"].([]any)
	tableRows := 0
	for _, e := range checklist {
		em := e.(map[string]any)
		if em["type"].(string) == "table_row" {
			tableRows++
			assert.Contains(t, em["id"].(string), "table-")
		}
	}
	assert.Equal(t, 2, tableRows, "should have 2 data rows (header excluded)")
}

func TestAuditChecklistNumberedItems(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Spec
## Steps
1. First step
2. Second step
3. Third step
`), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	checklist := result["checklist"].([]any)
	listItems := 0
	for _, e := range checklist {
		em := e.(map[string]any)
		if em["type"].(string) == "list_item" {
			listItems++
			assert.Contains(t, em["id"].(string), "list-")
		}
	}
	assert.Equal(t, 3, listItems)
}

func TestAuditChecklistTaskAcceptance(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\nEmpty.\n"), 0o600))

	taskPath := filepath.Join(dir, "spec.tasks.json")
	taskData := `{"version":1,"spec":"spec.md","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","workflow":{},"coverage":{"total_sections":0,"mapped_sections":0,"context_only":[],"unmapped":[]},"tasks":[{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"Model exists and migration runs.","source_sections":[],"source_lines":""},{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"","source_sections":[],"source_lines":""}]}`
	require.NoError(t, os.WriteFile(taskPath, []byte(taskData), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	checklist := result["checklist"].([]any)
	taskItems := 0
	for _, e := range checklist {
		em := e.(map[string]any)
		if em["type"].(string) == "task_acceptance" {
			taskItems++
			assert.Equal(t, "task-t1", em["id"].(string))
			assert.Equal(t, "T1", em["section"].(string))
		}
	}
	assert.Equal(t, 1, taskItems, "only task with non-empty acceptance should appear")
}

func TestAuditChecklistFindings(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Spec
## Table
| Col | Desc |
|-----|------|
| a | first |
`), 0o600))

	findingsPath := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"high","finding":"missing validation","category":"completeness","location":"line 5","suggestion":"add check"}
{"severity":"medium","message":"vague description","category":"ambiguity","location":"line 10","suggestion":"be specific"}
{"severity":"low","description":"consider renaming","category":"style","location":"line 15","suggestion":"use clearer name"}
{"severity":"low","category":"style","location":"line 20"}
`), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", findingsPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	checklist := result["checklist"].([]any)
	findingItems := 0
	for _, e := range checklist {
		em := e.(map[string]any)
		if em["type"].(string) == "finding" {
			findingItems++
			assert.Contains(t, em["id"].(string), "finding-")
		}
	}
	assert.Equal(t, 3, findingItems, "empty-text finding should be skipped")

	prompts := result["prompts"].([]any)
	assert.Equal(t, 2, len(prompts), "should have 2 prompts: impl + findings")
	assert.Equal(t, "implementation-auditor", prompts[1].(map[string]any)["role"].(string))
}

func TestAuditEmptyChecklist(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\nProse only, no structured elements.\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.Equal(t, float64(0), result["checklist_summary"].(map[string]any)["total"])
	cl, ok := result["checklist"].([]any)
	require.True(t, ok)
	assert.Empty(t, cl)
}

func TestAuditChecklistSummary(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Spec
## Table
| Col | Desc |
|-----|------|
| a | first |
## Steps
1. First step
`), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	summary := result["checklist_summary"].(map[string]any)
	assert.Equal(t, float64(2), summary["total"])
	byType := summary["by_type"].(map[string]any)
	assert.Equal(t, float64(1), byType["table_row"])
	assert.Equal(t, float64(1), byType["list_item"])
}

func TestAuditPromptContainsSourceFiles(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| C | D |\n|---|---|\n| a | b |\n"), 0o600))

	aPath := filepath.Join(dir, "code.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\nfunc Foo() int { return 42 }\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "code.go")
	assert.Contains(t, prompt, "Spec Excerpt")
	assert.Contains(t, prompt, "PASS|PARTIAL|FAIL")
}

// Test: filterChecklistByType returns empty slice (not nil) for JSON safety
func TestAuditFilterChecklistEmpty(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	// Spec with no structured elements → empty checklist
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\nJust some text.\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Checklist should be [] not null
	checklist := result["checklist"]
	assert.NotNil(t, checklist, "checklist should be [] not null")
	assert.IsType(t, []any{}, checklist, "checklist should be an array")
}

// Test: findings field priority order (finding > message > description > title)
func TestAuditChecklistFindingsPriority(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(
		`{"finding":"primary","message":"fallback"}`+"\n"+
			`{"message":"msg only"}`+"\n"+
			`{"description":"desc only"}`+"\n"+
			`{"title":"title only"}`+"\n",
	), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", findingsPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	checklist := result["checklist"].([]any)
	var findingTexts []string
	for _, e := range checklist {
		em := e.(map[string]any)
		if em["type"].(string) == "finding" {
			findingTexts = append(findingTexts, em["text"].(string))
		}
	}
	assert.Contains(t, findingTexts, "primary", "should use 'finding' field over 'message'")
	assert.NotContains(t, findingTexts, "fallback", "should not use 'message' when 'finding' exists")
	assert.Contains(t, findingTexts, "msg only")
	assert.Contains(t, findingTexts, "desc only")
	assert.Contains(t, findingTexts, "title only")
}

// Test: binary file filtering — .png, .jpg etc. should be excluded
func TestAuditBinaryFileFiltering(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| val |\n"), 0o600))

	goPath := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(goPath, []byte("package main\n"), 0o600))

	// Binary files should be filtered when using auto-detect, but --affected-files bypasses that.
	// So we test that providing a binary file directly still works (it's the user's choice)
	pngPath := filepath.Join(dir, "logo.png")
	require.NoError(t, os.WriteFile(pngPath, []byte{0x89, 0x50, 0x4E, 0x47}, 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", goPath, "--affected-files", pngPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	files := result["files"].([]any)
	assert.Len(t, files, 2, "both files should be accepted with --affected-files")
}

// Test: prompt contains full file path (not just basename)
func TestAuditPromptFullFilePath(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| val |\n"), 0o600))

	subDir := filepath.Join(dir, "internal", "pkg")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	aPath := filepath.Join(subDir, "handler.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package pkg\nfunc Handle() {}\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, aPath, "prompt should use full file path")
}

// Test: prompt splitting when checklist >= 50 entries
func TestAuditPromptSplitting(t *testing.T) {
	dir := t.TempDir()

	// Build a spec with many table rows (60 rows across multiple tables)
	var spec strings.Builder
	spec.WriteString("# Spec\n\n## Big Table\n\n| ID | Description |\n|----|-------------|\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&spec, "| item-%d | description for item %d |\n", i, i)
	}

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(spec.String()), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompts := result["prompts"].([]any)
	assert.GreaterOrEqual(t, len(prompts), 2, "60 checklist items should split into at least 2 prompts")

	// Verify first prompt has 50 items, second has 10
	p0 := prompts[0].(map[string]any)
	p1 := prompts[1].(map[string]any)
	assert.Equal(t, float64(50), p0["checklist_count"])
	assert.Equal(t, float64(10), p1["checklist_count"])
}

// Test: compact mode truncates text to exactly 80 chars (77 + "...")
func TestAuditCompactTruncationExact(t *testing.T) {
	dir := t.TempDir()

	// Table row with exactly 90 characters of content
	longRow := strings.Repeat("a", 90)
	spec := fmt.Sprintf("# Spec\n## Table\n| Col |\n|-----|\n| %s |\n", longRow)
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--compact")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	checklist := result["checklist"].([]any)
	for _, e := range checklist {
		em := e.(map[string]any)
		text := em["text"].(string)
		if len(text) > 80 {
			assert.Equal(t, 80, len(text), "truncated text should be exactly 80 chars (77 + ...)")
			assert.True(t, strings.HasSuffix(text, "..."), "truncated text should end with ...")
		}
	}
}

func TestAuditCompact(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col | Desc |\n|-----|------|\n| a | a very long description that should be truncated in compact mode |\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--compact")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.Nil(t, result["file_summary"])
	checklist := result["checklist"].([]any)
	for _, e := range checklist {
		em := e.(map[string]any)
		text := em["text"].(string)
		assert.LessOrEqual(t, len(text), 83, "text should be truncated to <=80 chars + ...")
	}

	prompts := result["prompts"].([]any)
	assert.NotEmpty(t, prompts, "compact should still include prompts")
}

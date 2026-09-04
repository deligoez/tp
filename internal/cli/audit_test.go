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
	t.Parallel()
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
	assert.Equal(t, "spec-coverage", result["prompts"].([]any)[0].(map[string]any)["role"].(string))
}

func TestAuditNoAffectedFilesNoGit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\nEmpty.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath)
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "not in a git repo")
}

func TestAuditAffectedFileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", "/nonexistent")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "not found")
}

func TestAuditAffectedFileDirectory(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestAuditGitFailureWarnsInsteadOfClaimingZeros: a git invocation that FAILS
// is not the same as a comparison that found nothing. Every caller turns the
// failure into a zero value — an empty file list, an empty diff-stat map — and
// a zero value is indistinguishable from a genuinely unchanged tree once it
// reaches an auditor prompt as fact. An unknown revision is the everyday case:
// the run must name it rather than report "no changed files" and send the
// caller looking for missing work instead of at the revision they typed.
func TestAuditGitFailureWarnsInsteadOfClaimingZeros(t *testing.T) {
	t.Parallel()
	dir, _ := newAuditRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--base", "no-such-tag")
	require.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, "warning: git diff --name-only no-such-tag...HEAD failed",
		"a failed selection lookup must not render as \"nothing changed\" without a word")
	assert.Contains(t, stderr, "unknown revision",
		"the advisory names git's own diagnosis, so the caller can see it is their revision")
}

// TestAuditDiffStatsMeasureTheSelectedRange is the other half of the guard
// above, and the dominant real case: tp's one-task-one-commit rule means the
// work being audited is already COMMITTED. File selection compares
// <tag>...HEAD, but a diff-stat lookup with no range compares the working tree
// — which on a committed tree SUCCEEDS with empty output. git never fails, so
// no warning fires, and every role is handed "(diff: +0/-0)" as a measured fact
// about files that were selected precisely because they changed. The stats must
// describe the same comparison the selection made.
func TestAuditDiffStatsMeasureTheSelectedRange(t *testing.T) {
	t.Parallel()
	dir, _ := newAuditRepo(t)
	git(t, dir, "tag", "v0.0.1")

	// 7 added lines, 0 deleted, committed — nothing left in the working tree.
	body := "package main\n\nfunc a1() {}\nfunc a2() {}\nfunc a3() {}\nfunc a4() {}\nfunc a5() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(body), 0o600))
	git(t, dir, "add", "a.go")
	git(t, dir, "commit", "-m", "add a.go")

	cases := []struct {
		name string
		args []string
	}{
		{"explicit base", []string{"audit", "spec.md", "--base", "v0.0.1"}},
		{"auto-detect since latest tag", []string{"audit", "spec.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runTP(t, dir, tc.args...)
			require.Equal(t, 0, code, stderr)

			var out map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &out))

			seen := 0
			for _, p := range out["prompts"].([]any) {
				for _, af := range p.(map[string]any)["affected_files"].([]any) {
					am := af.(map[string]any)
					if am["path"] != "a.go" {
						continue
					}
					seen++
					assert.Equal(t, "+7/-0", am["diff_summary"],
						"a committed 7-line addition must not reach a role as unchanged")
				}
			}
			require.NotZero(t, seen, "a.go must appear in the selected files")
		})
	}
}

// TestAuditCallerSuppliedFilesCarryNoUnmeasuredDiff is the third form of the
// same defect: the diff stats reproduce the AUTO-DETECT comparison, but
// --affected-files REPLACED the file universe, so that comparison need not
// contain the caller's paths at all. Here a.go changed before the latest tag,
// which puts it outside every range the audit compares — git succeeds, the path
// is simply absent from the numstat output, and the old fallback stated
// "+0/-0" about a file nothing measured. Nothing measured it, so nothing is
// claimed: no diff annotation on the prompt line, empty diff_summary in the
// payload.
func TestAuditCallerSuppliedFilesCarryNoUnmeasuredDiff(t *testing.T) {
	t.Parallel()
	dir, _ := newAuditRepo(t)

	body := "package main\n\nfunc a1() {}\nfunc a2() {}\nfunc a3() {}\nfunc a4() {}\nfunc a5() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(body), 0o600))
	git(t, dir, "add", "a.go")
	git(t, dir, "commit", "-m", "add a.go")
	// The tag comes AFTER the change, so <latest tag>...HEAD, the working tree
	// and the index are all empty: no range the audit compares covers a.go.
	git(t, dir, "tag", "v0.0.1")

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "a.go")
	require.Equal(t, 0, code, stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))

	seen := 0
	for _, p := range out["prompts"].([]any) {
		pm := p.(map[string]any)
		for _, af := range pm["affected_files"].([]any) {
			am := af.(map[string]any)
			if am["path"] != "a.go" {
				continue
			}
			seen++
			assert.Equal(t, "", am["diff_summary"],
				"an unmeasured file reports no churn rather than a zero one")
		}
		prompt := pm["prompt"].(string)
		assert.NotContains(t, prompt, "+0/-0",
			"role %s was handed an unmeasured zero as measured fact", pm["role"])
		assert.NotContains(t, prompt, "a.go (diff:",
			"role %s carries a diff annotation for a file no comparison covered", pm["role"])
		assert.Contains(t, prompt, "- a.go\n",
			"role %s still names the file, just without a churn claim", pm["role"])
	}
	require.NotZero(t, seen, "a.go must appear in the selected files")
}

func TestAuditNoSpecArg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "audit")
	assert.Equal(t, 2, code, "a missing spec is a usage error (exit 2), consistent with tp review")
	assert.Contains(t, stderr, "spec path required")
}

func TestAuditSpecNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "audit", "/nonexistent/spec.md", "--affected-files", "x.go")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "not found")
}

// TestAuditSpecNotFoundHint: a mistyped spec path used to inherit the code-3
// default hint, which is task-file advice ("run 'tp use <file>' … 'tp init
// <spec>'") — the wrong object entirely. The --affected-files branch in the
// same command already overrides that hint; the spec path gets its own too.
func TestAuditSpecNotFoundHint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "audit", "/nonexistent/spec.md", "--affected-files", "x.go")
	require.Equal(t, 3, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "spec path", "the hint names the spec path the caller typed")
	assert.NotContains(t, hint, "tp use", "task-file advice is the wrong object for a spec-path typo")
	assert.NotContains(t, hint, "tp init", "task-file advice is the wrong object for a spec-path typo")
}

func TestAuditChecklistTableRows(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	// Findings route into the spec-coverage prompt's checklist (§1.2)
	prompts := result["prompts"].([]any)
	specPrompt := prompts[0].(map[string]any)
	assert.Equal(t, "spec-coverage", specPrompt["role"].(string))
	findingInPrompt := 0
	for _, item := range specPrompt["checklist_items"].([]any) {
		if item.(map[string]any)["type"].(string) == "finding" {
			findingInPrompt++
		}
	}
	assert.Equal(t, 3, findingInPrompt, "finding items land in spec-coverage")
}

func TestAuditEmptyChecklist(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	assert.Contains(t, prompt, "## Output Schema")
	assert.Contains(t, prompt, "one of PASS, PARTIAL, FAIL")
}

// Test: filterChecklistByType returns empty slice (not nil) for JSON safety
func TestAuditFilterChecklistEmpty(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	dir := t.TempDir()

	// Build a spec with many table rows (60 rows across multiple tables)
	var spec strings.Builder
	spec.WriteString("# Spec\n\n## Big Table\n\n| ID | Description |\n|----|-------------|\n")
	for i := range 60 {
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

	// v0.23.0 emits one prompt per role; spec items are no longer split
	prompts := result["prompts"].([]any)
	p0 := prompts[0].(map[string]any)
	assert.Equal(t, "spec-coverage", p0["role"].(string))
	assert.Equal(t, float64(60), p0["checklist_count"], "all spec items stay in the single spec-coverage prompt")
}

// Test: compact mode truncates text to exactly 80 chars (77 + "...")
func TestAuditCompactTruncationExact(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// An unreadable findings file (here: a directory, which os.ReadFile cannot
// read as a regular file) is a real read error, not a missing file — it must
// exit 3 (file) with a hint naming the findings path.
func TestAuditFindingsUnreadableDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	findingsDir := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.MkdirAll(findingsDir, 0o755))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", findingsDir)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(3), e["code"], "unreadable findings must exit 3 (file)")
	assert.Contains(t, e["error"], "cannot read findings file")
	assert.Contains(t, e["error"], findingsDir, "error must name the findings path")
	assert.Equal(t, 3, code)
}

// An absent findings file used to be treated as optional here; it is a typo
// that silently empties the round's finding set, so it now aborts. The
// replacement contract lives in TestAuditMissingFindingsFileFailsLoudly
// (audit_findings_path_test.go).

// A line past ndjsonLineCap in a --findings NDJSON aborts. This REVERSES the
// v0.28.0 contract this test used to pin (exit 0, warn on stderr, drop the rows
// after it), and the reversal is deliberate: these rows become the
// finding-verification checklist, so dropped rows are findings the audit never
// asks about while the round still records — the false-clean class every
// sibling NDJSON reader was swept for. The warning was also weaker than it
// read, since output.Notice honours --quiet: exit 0, empty stderr, a quietly
// shorter checklist. No reader was left on the warn contract: v0.34.0 §4 moved
// tp add --bulk and tp set --bulk to the same abort (add.go, set.go), so this
// is the shared rule rather than this reader's exception.
func TestAuditFindingsOverlongLineAborts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	ndjson := `{"finding":"kept before the over-long line"}` + "\n" +
		strings.Repeat("x", 2*1024*1024) + "\n" +
		`{"finding":"dropped after the over-long line"}` + "\n"
	require.NoError(t, os.WriteFile(findingsPath, []byte(ndjson), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", findingsPath)
	require.Equal(t, 3, code, "a truncated findings read is a file error, not a shorter checklist")
	assert.Contains(t, stderr, findingsPath, "the error must name the findings path")
	assert.NotContains(t, stdout, "kept before the over-long line",
		"no checklist is emitted from a findings file tp could not read whole")

	// --quiet is what made the old advisory unusable: it silenced the only
	// signal while the checklist still shrank. The abort must survive it.
	_, quietStderr, quietCode := runTP(t, dir, "audit", specPath, "--affected-files", aPath,
		"--findings", findingsPath, "--quiet")
	assert.Equal(t, 3, quietCode, "--quiet must not turn the abort back into a silent drop")
	assert.NotEmpty(t, quietStderr, "--quiet silences advisories, not errors")

	// Under the cap the same three rows read normally, so the abort is the
	// exception and not the new rule.
	okPath := filepath.Join(dir, "ok.ndjson")
	okNDJSON := `{"finding":"kept before the over-long line"}` + "\n" +
		`{"finding":"` + strings.Repeat("x", 200*1024) + `"}` + "\n" +
		`{"finding":"dropped after the over-long line"}` + "\n"
	require.NoError(t, os.WriteFile(okPath, []byte(okNDJSON), 0o600))

	okStdout, okStderr, okCode := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", okPath)
	require.Equal(t, 0, okCode, "a 200KB row is under the shared cap: %s", okStderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(okStdout), &result))
	keepSeen, tailSeen := false, false
	for _, e := range result["checklist"].([]any) {
		em := e.(map[string]any)
		if em["type"].(string) == "finding" {
			switch em["text"].(string) {
			case "kept before the over-long line":
				keepSeen = true
			case "dropped after the over-long line":
				tailSeen = true
			}
		}
	}
	assert.True(t, keepSeen, "the first finding reaches the checklist")
	assert.True(t, tailSeen, "so does the one after the 200KB row — nothing is dropped under the cap")
}

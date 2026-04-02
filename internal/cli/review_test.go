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

func setupPerspectiveTestDir(t *testing.T, extraFiles map[string]string) (specPath, docsDir string) {
	t.Helper()
	dir := t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Batch Closing Feature\n\n## Commands\n\n### tp done --batch\nClose multiple tasks from NDJSON file.\n\n## Validation\n1. Exit code 2 on invalid perspective\n2. Exit code 3 on missing docs path\n"), 0o600))

	docsDir = filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "guide"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "reference"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.md"), []byte("# My Project\n\n## Features\n- tp plan\n- tp done\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "guide", "getting-started.md"), []byte("# Getting Started\n\nRun `tp plan` to see tasks.\n\n## Commands\n- tp plan\n- tp done\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "reference", "commands.md"), []byte("# Commands Reference\n\n## tp done\nClose a single task.\n"), 0o600))

	for path, content := range extraFiles {
		fullPath := filepath.Join(docsDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
	}

	return specPath, docsDir
}

func TestReviewBasic(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Feature

## Problem
Users need X.

## Solution

| Component | Change |
|-----------|--------|
| Model | Add field |
| API | New endpoint |

## Implementation Order

1. Add model field
2. Create endpoint
3. Write tests
`), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code, "review should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Should have 3 prompts
	prompts := result["prompts"].([]any)
	assert.Len(t, prompts, 3)

	// Check roles
	roles := make(map[string]bool)
	for _, p := range prompts {
		pm := p.(map[string]any)
		roles[pm["role"].(string)] = true
		// Each prompt should contain spec content
		assert.Contains(t, pm["prompt"].(string), "Users need X")
	}
	assert.True(t, roles["implementer"])
	assert.True(t, roles["tester"])
	assert.True(t, roles["architect"])

	// Review loop
	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(2), loop["max_rounds"])
}

func TestReviewStructuredElementsInPrompts(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Spec

## Rules

| Condition | Action |
|-----------|--------|
| A | Do X |
| B | Do Y |
| C | Do Z |

## Steps

1. First step
2. Second step
3. Third step
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Structured elements should be present
	se := result["structured_elements"].(map[string]any)
	assert.Equal(t, float64(3), se["total_table_rows"])
	assert.Equal(t, float64(3), se["total_numbered_items"])

	// Implementer prompt should reference the table
	prompts := result["prompts"].([]any)
	implPrompt := prompts[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, implPrompt, "Table 'Rules'")
	assert.Contains(t, implPrompt, "3 rows")

	// Should reference the numbered list
	assert.Contains(t, implPrompt, "List 'Steps'")
}

func TestReviewFindingFormatInPrompts(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Every prompt should contain the finding format
	for _, p := range result["prompts"].([]any) {
		prompt := p.(map[string]any)["prompt"].(string)
		assert.Contains(t, prompt, "severity")
		assert.Contains(t, prompt, "category")
		assert.Contains(t, prompt, "finding")
		assert.Contains(t, prompt, "suggestion")
	}
}

func TestReviewEmptySpec(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Empty\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Should still produce 3 prompts (with generic questions)
	prompts := result["prompts"].([]any)
	assert.Len(t, prompts, 3)

	// Structured elements should be zero
	se := result["structured_elements"].(map[string]any)
	assert.Equal(t, float64(0), se["total_table_rows"])
	assert.Equal(t, float64(0), se["total_numbered_items"])
}

func TestReviewArchitectCrossReference(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(`# Spec

## What Gets Added
1. Component A
2. Component B

## Implementation Order
1. Step one
2. Step two
3. Step three
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Architect prompt should cross-reference the two lists
	archPrompt := result["prompts"].([]any)[2].(map[string]any)["prompt"].(string)
	assert.Contains(t, archPrompt, "Cross-reference")
	assert.Contains(t, archPrompt, "What Gets Added")
	assert.Contains(t, archPrompt, "Implementation Order")
}

func TestReviewRound1BackwardCompat(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(1), loop["round"])
	assert.Equal(t, float64(0), loop["previous_findings"])

	for _, p := range result["prompts"].([]any) {
		prompt := p.(map[string]any)["prompt"].(string)
		assert.NotContains(t, prompt, "Previous Review Round")
	}
}

func TestReviewRound2WithFindings(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"high","category":"completeness","location":"## Problem","finding":"Missing edge case","suggestion":"Add edge case section"}
{"severity":"medium","category":"ambiguity","location":"line 2","finding":"Vague requirement","suggestion":"Be specific"}
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--round", "2", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(2), loop["round"])
	assert.Equal(t, float64(2), loop["previous_findings"])

	for _, p := range result["prompts"].([]any) {
		prompt := p.(map[string]any)["prompt"].(string)
		assert.Contains(t, prompt, "Previous Review Round")
		assert.Contains(t, prompt, "[HIGH] completeness")
		assert.Contains(t, prompt, "[MED] ambiguity")
		assert.Contains(t, prompt, "review round 2")
		assert.Contains(t, prompt, "only report NEW issues")
	}
}

func TestReviewRound2WithoutFindings(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--round", "2", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(2), loop["round"])
	assert.Equal(t, float64(0), loop["previous_findings"])
}

func TestReviewRound0(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "--round", "0", specPath)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "round must be")
}

func TestReviewFindingsFileNotFound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "--findings", "/nonexistent/file.ndjson", specPath)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "findings file not found")
}

func TestReviewFindingsDedup(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"high","category":"completeness","location":"## Problem","finding":"Missing edge case","suggestion":"Add section"}
{"severity":"medium","category":"completeness","location":"## Problem","finding":"Another issue same location","suggestion":"Fix it"}
{"severity":"low","category":"ambiguity","location":"## Problem","finding":"Same location different category","suggestion":"Clarify"}
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(2), loop["previous_findings"])
}

func TestReviewFindingsCappedAt50(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	var lines []byte
	for i := 0; i < 55; i++ {
		lines = append(lines, []byte(fmt.Sprintf(`{"severity":"low","category":"completeness","location":"line %d","finding":"Issue number %d","suggestion":"Fix it"}
`, i, i))...)
	}
	require.NoError(t, os.WriteFile(findingsPath, lines, 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "5 more (omitted for brevity)")
	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(55), loop["previous_findings"])
}

func TestReviewFindingsLongText(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	longFinding := strings.Repeat("x", 100)
	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(fmt.Sprintf(`{"severity":"high","category":"completeness","location":"## Problem","finding":"%s","suggestion":"Fix it"}
`, longFinding)), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "xxx...")
}

func TestReviewFindingsInvalidLines(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`not json at all
{"severity":"high","category":"completeness","location":"## Problem","finding":"Valid finding","suggestion":"Fix it"}
also not json
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(1), loop["previous_findings"])
}

func TestReviewEmptyFindingsFile(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(""), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	for _, p := range result["prompts"].([]any) {
		prompt := p.(map[string]any)["prompt"].(string)
		assert.NotContains(t, prompt, "Previous Review Round")
	}

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(0), loop["previous_findings"])
}

func TestReviewRound1WithFindings(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"high","category":"completeness","location":"## Problem","finding":"Missing edge case","suggestion":"Fix it"}
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--round", "1", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(1), loop["round"])
	assert.Equal(t, float64(1), loop["previous_findings"])

	implPrompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, implPrompt, "Previous Review Round")
	assert.Contains(t, implPrompt, "[HIGH] completeness")
	assert.NotContains(t, implPrompt, "review round 2")
	assert.Contains(t, implPrompt, "only report NEW issues")

	assert.Contains(t, loop["instruction"].(string), "--round 2 --findings")
}

func TestReviewSeveritySortOrder(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"low","category":"ambiguity","location":"L3","finding":"Low finding","suggestion":"Fix"}
{"severity":"critical","category":"consistency","location":"L1","finding":"Critical finding","suggestion":"Fix"}
{"severity":"high","category":"completeness","location":"L2","finding":"High finding","suggestion":"Fix"}
{"severity":"medium","category":"feasibility","location":"L4","finding":"Med finding","suggestion":"Fix"}
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	critIdx := strings.Index(prompt, "[CRIT]")
	highIdx := strings.Index(prompt, "[HIGH]")
	medIdx := strings.Index(prompt, "[MED]")
	lowIdx := strings.Index(prompt, "[LOW]")
	assert.Less(t, critIdx, highIdx, "CRIT should come before HIGH")
	assert.Less(t, highIdx, medIdx, "HIGH should come before MED")
	assert.Less(t, medIdx, lowIdx, "MED should come before LOW")
}

func TestReviewNegativeRound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "--round", "-1", specPath)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "round must be")
}

func TestReviewRound3Instruction(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--round", "3", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(3), loop["round"])
	assert.Contains(t, loop["instruction"].(string), "--round 4 --findings")

	implPrompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, implPrompt, "review round 3")
}

func TestReviewMissingSeverityDefaultsToUnknown(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"category":"completeness","location":"## Problem","finding":"No severity field","suggestion":"Fix it"}
`), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "[???] completeness")
	assert.Equal(t, float64(1), result["review_loop"].(map[string]any)["previous_findings"])
}

func TestReviewPerspectiveDocBasic(t *testing.T) {
	specPath, docsDir := setupPerspectiveTestDir(t, nil)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", specPath, "--perspective", "documentation", "--docs-path", docsDir)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.Equal(t, "documentation", result["perspective"])
	assert.Equal(t, docsDir, result["docs_path"])

	prompts := result["prompts"].([]any)
	require.Len(t, prompts, 1)
	assert.Equal(t, "documentation-planner", prompts[0].(map[string]any)["role"])
}

func TestReviewPerspectiveTestBasic(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Feature\nNew feature.\n"), 0o600))

	testDir := filepath.Join(dir, "internal")
	require.NoError(t, os.MkdirAll(testDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "cli_review_test.go"), []byte("package cli\n\nfunc TestSomething(t *testing.T) {}\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath, "--perspective", "testing", "--test-path", testDir)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.Equal(t, "testing", result["perspective"])
	assert.Equal(t, testDir, result["test_path"])

	prompts := result["prompts"].([]any)
	require.Len(t, prompts, 1)
	assert.Equal(t, "test-planner", prompts[0].(map[string]any)["role"])
}

func TestReviewPerspectiveInvalid(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "invalid")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "invalid perspective")
}

func TestReviewPerspectiveMissingDocsPath(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "documentation")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--docs-path is required")
}

func TestReviewPerspectiveMissingTestPath(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "testing")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--test-path is required")
}

func TestReviewPerspectiveDocsPathNotFound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "documentation", "--docs-path", "/nonexistent")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "docs path not found")
}

func TestReviewPerspectiveTestPathNotFound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "testing", "--test-path", "/nonexistent")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "test path not found")
}

func TestReviewPerspectiveMutuallyExclusiveRound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "documentation", "--docs-path", dir, "--round", "2")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "mutually exclusive")
}

func TestReviewPerspectiveMutuallyExclusiveFindings(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))
	findingsPath := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"high","category":"x","location":"y","finding":"z","suggestion":"w"}
`), 0o600))

	_, stderr, code := runTP(t, dir, "review", specPath, "--perspective", "testing", "--test-path", dir, "--findings", findingsPath)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "mutually exclusive")
}

func TestReviewPerspectiveDefaultUnchanged(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Simple Spec\nDo the thing.\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.Empty(t, result["perspective"])
	prompts := result["prompts"].([]any)
	assert.Len(t, prompts, 3)
}

func TestReviewPerspectiveDocPromptContent(t *testing.T) {
	specPath, docsDir := setupPerspectiveTestDir(t, nil)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", specPath, "--perspective", "documentation", "--docs-path", docsDir)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	ds := result["docs_structure"].(map[string]any)
	assert.Equal(t, float64(3), ds["total_files"])
	assert.Equal(t, float64(3), ds["reviewed_files"])
	assert.Contains(t, ds["structure_map"].(string), "index.md")

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "Batch Closing Feature")
	assert.Contains(t, prompt, "A1")
	assert.Contains(t, prompt, "A5")
	assert.Contains(t, prompt, "create-page")
	assert.Contains(t, prompt, "fix-drift")
	assert.Contains(t, prompt, "update-config")
	assert.Contains(t, prompt, "add-crossref")
	assert.Contains(t, prompt, "update-index")
	assert.Contains(t, prompt, "must|should|could")

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, float64(1), loop["round"])
	assert.Equal(t, float64(1), loop["max_rounds"])
	assert.Equal(t, "single-pass plan generation", loop["convergence"])
}

func TestReviewPerspectiveTestPromptContent(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Feature\n\n## Commands\n\n| Flag | Type |\n|------|------|\n| --batch | string |\n\n## Acceptance\n1. Exit code 0 on valid input\n2. Exit code 2 on invalid input\n"), 0o600))

	testDir := filepath.Join(dir, "internal")
	require.NoError(t, os.MkdirAll(filepath.Join(testDir, "cli"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "cli", "review_test.go"), []byte("func TestReview(t *testing.T) {}\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", specPath, "--perspective", "testing", "--test-path", testDir)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	ts := result["test_structure"].(map[string]any)
	assert.Equal(t, float64(1), ts["total_files"])
	assert.Contains(t, ts["structure_map"].(string), "review_test.go")

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "Feature")
	assert.Contains(t, prompt, "T1")
	assert.Contains(t, prompt, "T7")
	assert.Contains(t, prompt, "create-test")
	assert.Contains(t, prompt, "update-test")
	assert.Contains(t, prompt, "create-fixture")
}

func TestReviewPerspectiveEmptyDocsDir(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	stdout, _, code := runTP(t, dir, "review", specPath, "--perspective", "documentation", "--docs-path", docsDir)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	ds := result["docs_structure"].(map[string]any)
	assert.Equal(t, float64(0), ds["total_files"])
	assert.Equal(t, float64(0), ds["reviewed_files"])

	prompts := result["prompts"].([]any)
	require.Len(t, prompts, 1)
	assert.Contains(t, prompts[0].(map[string]any)["prompt"].(string), "A1")
}

func TestReviewPerspectiveManyFilesCapped(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Feature\nFeature with many docs pages.\n"), 0o600))

	docsDir := filepath.Join(dir, "docs")
	for i := 0; i < 20; i++ {
		subDir := filepath.Join(docsDir, fmt.Sprintf("section%d", i))
		require.NoError(t, os.MkdirAll(subDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "page.md"), []byte(fmt.Sprintf("# Page %d\nFeature content here.\n", i)), 0o600))
	}

	stdout, _, code := runTP(t, dir, "review", specPath, "--perspective", "documentation", "--docs-path", docsDir)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	ds := result["docs_structure"].(map[string]any)
	assert.Equal(t, float64(20), ds["total_files"])
	assert.LessOrEqual(t, ds["reviewed_files"], float64(15))
}

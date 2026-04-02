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

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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

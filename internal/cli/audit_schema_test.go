package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditSchema_PromptFieldsAndOutputSchema(t *testing.T) {
	dir := t.TempDir()
	spec := "# Spec\n## Table\n| C | D |\n|---|---|\n| a | b |\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	prompts := out["prompts"].([]any)
	require.NotEmpty(t, prompts)
	p := prompts[0].(map[string]any)

	// auditPrompt has no Category field; gains checklist_items and affected_files
	_, hasCategory := p["category"]
	assert.False(t, hasCategory, "prompt-level category is removed in v0.23.0")
	items := p["checklist_items"].([]any)
	require.NotEmpty(t, items)
	item := items[0].(map[string]any)
	assert.Equal(t, "table-0-0", item["item_id"])
	assert.Equal(t, "table_row", item["type"])
	assert.NotEmpty(t, item["expected_evidence"], "expected_evidence is never empty")
	files := p["affected_files"].([]any)
	require.NotEmpty(t, files)
	assert.Contains(t, files[0].(map[string]any)["path"], "code.go")

	// §4 output schema embedded as prompt guidance, including the class row
	// and the category enum with resolution precedence
	text := p["prompt"].(string)
	assert.Contains(t, text, "## Output Schema")
	assert.Contains(t, text, `"42-58" (range) or "42" (single line)`)
	assert.Contains(t, text, "class: optional; kebab-case slug")
	assert.Contains(t, text, "security > concurrency > error-handling > correctness > contract")
}

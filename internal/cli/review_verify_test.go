package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVerifyTest(t *testing.T, findings string) (specPath, findingsPath string) {
	t.Helper()
	dir := t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## Section\n\nContent.\n"), 0o600))
	findingsPath = filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(findings), 0o600))
	return specPath, findingsPath
}

func TestReviewVerifyAllFixed(t *testing.T) {
	specPath, findingsPath := setupVerifyTest(t, `{"severity":"high","category":"completeness","location":"## A","finding":"missing X","resolved":{"status":"fixed","evidence":"added in section 2"}}
{"severity":"medium","category":"ambiguity","location":"## B","finding":"unclear Y","resolved":{"status":"fixed","evidence":"clarified"}}
`)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", "--verify", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompts := result["prompts"].([]any)
	assert.Len(t, prompts, 1, "verify should produce single prompt")

	pm := prompts[0].(map[string]any)
	assert.Equal(t, "verifier", pm["role"])
	assert.Equal(t, "verification", pm["category"])

	prompt := pm["prompt"].(string)
	assert.Contains(t, prompt, "2 were addressed")
	assert.Contains(t, prompt, "0 remain unresolved")
	assert.Contains(t, prompt, "Fixed findings to verify")
	assert.Contains(t, prompt, "added in section 2")
}

func TestReviewVerifyMixed(t *testing.T) {
	specPath, findingsPath := setupVerifyTest(t, `{"severity":"high","category":"completeness","location":"## A","finding":"fixed one","resolved":{"status":"fixed","evidence":"done"}}
{"severity":"medium","category":"ambiguity","location":"## B","finding":"wontfix one","resolved":{"status":"wontfix","evidence":"out of scope"}}
{"severity":"low","category":"consistency","location":"## C","finding":"still open"}
`)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", "--verify", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "1 were addressed")
	assert.Contains(t, prompt, "1 were marked won't-fix")
	assert.Contains(t, prompt, "1 remain unresolved")
	assert.Contains(t, prompt, "Fixed findings to verify")
	assert.Contains(t, prompt, "Won't-fix findings")
	assert.Contains(t, prompt, "Unresolved findings")
}

func TestReviewVerifyNoFindings(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, _, code := runTP(t, dir, "review", "--verify", specPath)
	assert.Equal(t, 2, code, "verify without --findings should exit 2")
}

func TestReviewVerifyModeField(t *testing.T) {
	specPath, findingsPath := setupVerifyTest(t, `{"severity":"high","category":"completeness","location":"## A","finding":"test","resolved":{"status":"fixed","evidence":"done"}}
`)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", "--verify", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	loop := result["review_loop"].(map[string]any)
	assert.Equal(t, "verification", loop["mode"])
	assert.Contains(t, loop["instruction"].(string), "review is complete")
}

func TestReviewVerifyWithAffectedFiles(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## Section\nContent.\n"), 0o600))
	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{"severity":"high","category":"completeness","location":"## A","finding":"test"}`+"\n"), 0o600))
	aPath := filepath.Join(dir, "code.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\nfunc Check() {}\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "--verify", "--findings", findingsPath, "--affected-files", aPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "Check", "affected file content should be in verify prompt")
}

func TestReviewVerifyEmptyCategories(t *testing.T) {
	// All unresolved — no fixed or wontfix sections
	specPath, findingsPath := setupVerifyTest(t, `{"severity":"high","category":"completeness","location":"## A","finding":"still open"}
`)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", "--verify", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.NotContains(t, prompt, "Fixed findings to verify")
	assert.NotContains(t, prompt, "Won't-fix findings")
	assert.Contains(t, prompt, "Unresolved findings")
}

func TestReviewVerifyRegressionGuidance(t *testing.T) {
	specPath, findingsPath := setupVerifyTest(t, `{"severity":"high","category":"completeness","location":"## A","finding":"test","resolved":{"status":"fixed","evidence":"done"}}
`)

	stdout, _, code := runTP(t, filepath.Dir(specPath), "review", "--verify", "--findings", findingsPath, specPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	prompt := result["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "regression", "verify prompt should include regression guidance")
}

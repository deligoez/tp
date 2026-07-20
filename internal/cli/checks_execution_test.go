package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecksExecution_PromptExclusion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow",
		`checks=[{"class":"pass-class","cmd":"true"},{"class":"fail-class","cmd":"echo tail-marker; exit 1"}]`)
	require.Equal(t, 0, code)

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "check failures never abort prompt generation: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))

	// Registered classes listed in every prompt
	prompts := out["prompts"].([]any)
	require.Len(t, prompts, 3)
	for _, p := range prompts {
		text := p.(map[string]any)["prompt"].(string)
		assert.Contains(t, text, "do NOT report findings of these classes")
		assert.Contains(t, text, "pass-class")
		assert.Contains(t, text, "fail-class")
	}

	// Failed check appears in mechanical_checks with its output tail
	checks := out["mechanical_checks"].([]any)
	require.Len(t, checks, 2)
	pass := checks[0].(map[string]any)
	assert.Equal(t, true, pass["passed"])
	_, hasTail := pass["output_tail"]
	assert.False(t, hasTail, "output_tail only for failed checks")
	fail := checks[1].(map[string]any)
	assert.Equal(t, false, fail["passed"])
	assert.Contains(t, fail["output_tail"], "tail-marker")

	// Instruction tells the agent to fix failures first
	loop := out["review_loop"].(map[string]any)
	assert.Contains(t, loop["instruction"], "fix those failures before spawning sub-agents")
}

func TestChecksExecution_RunsUnderNoState(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", `checks=[{"class":"marker-class","cmd":"echo ran >> check_ran.txt"}]`)
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)

	data, err := os.ReadFile(filepath.Join(dir, "check_ran.txt"))
	require.NoError(t, err, "checks are workflow-derived and run even under --no-state")
	assert.Contains(t, string(data), "ran")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.NotEmpty(t, out["mechanical_checks"])
	prompts := out["prompts"].([]any)
	assert.Contains(t, prompts[0].(map[string]any)["prompt"], "marker-class")
}

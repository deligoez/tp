package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const proseSpec = `---
tp:
  domain: prose
  lens:
    all:
      - "Does any chapter leak a plot point?"
    implementer:
      - "Can each section be written without inventing facts?"
    tester:
      - "Is every gate condition checkable?"
---
# Book Outline
## 1. Chapter One
outline content
`

func reviewPromptsByRole(t *testing.T, stdout string) map[string]string {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	byRole := map[string]string{}
	for _, p := range out["prompts"].([]any) {
		m := p.(map[string]any)
		byRole[m["role"].(string)] = m["prompt"].(string)
	}
	return byRole
}

// TestLensInjection_DomainSwitch: a non-software domain swaps personas and
// drops the three software questions; lens questions are appended in order.
func TestLensInjection_DomainSwitch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(proseSpec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)

	// Persona switches per the 12.3 table
	assert.Contains(t, byRole["implementer"], "You must execute this spec exactly as written, starting tomorrow")
	assert.NotContains(t, byRole["implementer"], "senior engineer")
	assert.Contains(t, byRole["tester"], "You must verify every claim in this spec with a pass/fail procedure")
	assert.Contains(t, byRole["architect"], "You review this spec for internal consistency and structural soundness")

	// Three software questions dropped
	assert.NotContains(t, byRole["implementer"], "happy path fails")
	assert.NotContains(t, byRole["architect"], "backward compatibility section")
	assert.NotContains(t, byRole["architect"], "performance or scalability")

	// Domain-neutral questions stay
	assert.Contains(t, byRole["tester"], "vague language")
	assert.Contains(t, byRole["tester"], "interpret any requirement differently")
	assert.Contains(t, byRole["architect"], "Do any sections contradict each other?")

	// Lens injection: all everywhere, role-specific only in its role, order
	// hardcoded -> all -> role-specific
	for _, role := range []string{"implementer", "tester", "architect"} {
		assert.Contains(t, byRole[role], "Does any chapter leak a plot point?", "%s carries lens.all", role)
	}
	assert.Contains(t, byRole["implementer"], "Can each section be written without inventing facts?")
	assert.NotContains(t, byRole["tester"], "Can each section be written without inventing facts?")
	assert.Contains(t, byRole["tester"], "Is every gate condition checkable?")
	assert.NotContains(t, byRole["architect"], "Is every gate condition checkable?")

	implPrompt := byRole["implementer"]
	allIdx := strings.Index(implPrompt, "Does any chapter leak a plot point?")
	roleIdx := strings.Index(implPrompt, "Can each section be written without inventing facts?")
	hardIdx := strings.Index(implPrompt, "implicit assumptions")
	assert.Less(t, hardIdx, allIdx, "hardcoded before all")
	assert.Less(t, allIdx, roleIdx, "all before role-specific")
}

func TestDomainLens_SoftwareDomainUnchanged(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)

	assert.Contains(t, byRole["implementer"], "senior engineer who must implement this spec tomorrow")
	assert.Contains(t, byRole["implementer"], "happy path fails")
	assert.Contains(t, byRole["architect"], "backward compatibility section")
	assert.Contains(t, byRole["architect"], "performance or scalability")
}

func TestDomainLens_RegressionGetsAllLens(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(proseSpec), 0o600))
	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(strings.Replace(proseSpec, "outline content", "changed content", 1)), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)
	require.Contains(t, byRole, "regression")
	assert.Contains(t, byRole["regression"], "Does any chapter leak a plot point?", "lens.all appended to the regression prompt")
	assert.NotContains(t, byRole["regression"], "Can each section be written without inventing facts?", "role-specific lens stays out of regression")
}

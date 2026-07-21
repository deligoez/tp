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

// TestReview_ProseDomainEmitsProsePanel: a prose-domain spec emits the leaner
// prose corpus panel (coherence + soundness), not the swapped software personas
// — the persona swap is retired and domain only selects the corpus (§6.2, §6.3).
func TestReview_ProseDomainEmitsProsePanel(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(proseSpec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)

	// The prose corpus panel replaces the three swapped software personas.
	assert.Contains(t, byRole, "coherence")
	assert.Contains(t, byRole, "soundness")
	assert.NotContains(t, byRole, "implementer")
	assert.NotContains(t, byRole, "tester")
	assert.NotContains(t, byRole, "architect")

	// The role's failure lens now comes from its corpus instructions/focus.
	assert.Contains(t, byRole["coherence"], "structural and narrative continuity")
	assert.Contains(t, byRole["soundness"], "expository soundness")
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

func TestDomainLens_RegressionRejectsLens(t *testing.T) {
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
	// Regression accepts no lens/override (§5.2, §10.4); the active prose review
	// roles receive lens.all via the back-compat shim.
	assert.NotContains(t, byRole["regression"], "Does any chapter leak a plot point?", "regression rejects lens.all")
	assert.Contains(t, byRole["coherence"], "Does any chapter leak a plot point?", "lens.all fans out to the active review roles")
}

// TestReview_CorpusDrivenEmission: a user reviewer corpus replaces the embedded
// panel — tp emits one prompt per corpus role, carrying its instructions and
// focus (§7.1).
func TestReview_CorpusDrivenEmission(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "transaction-integrity.json"),
		[]byte(`{"id":"transaction-integrity","title":"Transaction Integrity","instructions":"You hunt for non-atomic state transitions.","focus":["Is every write rolled back on error?"]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)

	assert.Contains(t, byRole, "transaction-integrity", "the user corpus replaces the embedded panel")
	assert.NotContains(t, byRole, "implementer")
	assert.Contains(t, byRole["transaction-integrity"], "non-atomic state transitions", "role instructions are emitted")
	assert.Contains(t, byRole["transaction-integrity"], "Is every write rolled back on error?", "role focus is emitted")
}

// TestReview_RegressionAppendedNotCorpus: after a recorded round and a spec
// change, review appends the built-in regression role — never a corpus file —
// alongside the corpus panel (§5.2, §7.1).
func TestReview_RegressionAppendedNotCorpus(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "solo.json"),
		[]byte(`{"id":"solo","title":"Solo","instructions":"You review.","focus":["Q?"]}`), 0o600))
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\ncontent\n"), 0o600))

	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\nchanged content\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "solo", "the single corpus reviewer is emitted")
	assert.Contains(t, byRole, "regression", "regression is appended as a built-in, non-corpus role")
}

// TestReview_DomainDoesNotSwapPersona: a reviewer role applying to every domain
// emits its persona verbatim regardless of the spec's domain — domain no longer
// swaps Go personas; it only selects and filters the corpus (§6.2, §10.1).
func TestReview_DomainDoesNotSwapPersona(t *testing.T) {
	for _, domain := range []string{"software", "prose"} {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		revDir := filepath.Join(dir, ".tp", "reviewers")
		require.NoError(t, os.MkdirAll(revDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(revDir, "universal.json"),
			[]byte(`{"id":"universal","title":"U","instructions":"VERBATIM PERSONA TEXT"}`), 0o600))

		spec := "# Spec\ncontent\n"
		if domain == "prose" {
			spec = "---\ntp:\n  domain: prose\n---\n# Spec\ncontent\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

		stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
		require.Equal(t, 0, code, "domain %s stderr: %s", domain, stderr)
		byRole := reviewPromptsByRole(t, stdout)
		require.Contains(t, byRole, "universal", "domain %s selects the no-domains role", domain)
		assert.Contains(t, byRole["universal"], "VERBATIM PERSONA TEXT", "persona is not swapped by domain %s", domain)
	}
}

// TestReview_FrontmatterOverrideFocus: a tp.review_roles override appends its
// focus to the matching corpus role's focus at emission, project focus first
// (§10.2, §10.3).
func TestReview_FrontmatterOverrideFocus(t *testing.T) {
	dir := t.TempDir()
	spec := "---\ntp:\n  review_roles:\n    implementer:\n      focus:\n        - \"OVERRIDE FOCUS QUESTION\"\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole["implementer"], "OVERRIDE FOCUS QUESTION", "the override focus is appended")
	assert.Contains(t, byRole["implementer"], "happy path fails", "the corpus focus is retained (additive)")
}

// TestReview_OverrideUnknownIDIgnored: an override id matching no active role is
// ignored — its focus reaches no emitted role (§10.2). The warning text itself is
// covered by the engine test TestResolveOverrideFocus_UnknownID.
func TestReview_OverrideUnknownIDIgnored(t *testing.T) {
	dir := t.TempDir()
	spec := "---\ntp:\n  review_roles:\n    ghost:\n      focus:\n        - \"GHOST QUESTION\"\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)
	for role, prompt := range byRole {
		assert.NotContains(t, prompt, "GHOST QUESTION", "the ghost override must not reach role %s", role)
	}
}

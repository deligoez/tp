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

// TestReview_EnabledTrueIsNoOp: enabled: true is accepted as a no-op — the role
// stays in the emitted panel with its override focus layered onto the corpus
// focus, exactly as an override without the key behaves (§2.1, test 3).
func TestReview_EnabledTrueIsNoOp(t *testing.T) {
	dir := t.TempDir()
	spec := "---\ntp:\n  review_roles:\n    implementer:\n      enabled: true\n      focus:\n        - \"ENABLED TRUE FOCUS\"\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)
	require.Contains(t, byRole, "implementer", "enabled: true leaves the role active")
	assert.Contains(t, byRole["implementer"], "ENABLED TRUE FOCUS", "the override focus is still layered")
	assert.Contains(t, byRole["implementer"], "happy path fails", "the corpus focus is retained")
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

// TestReview_DeactivatingEveryUserRoleDoesNotFallBack: the enabled: false drop
// is applied outside ResolveActiveCorpus and after its domain filtering, so a
// spec deactivating every user reviewer leaves the panel empty and tp refuses
// (exit 2) instead of tripping the empty-panel fallback to the embedded default
// corpus (§2.3, test 11). An implementation that dropped the role inside
// ResolveActiveCorpus would exit 0 and emit the embedded panel instead.
func TestReview_DeactivatingEveryUserRoleDoesNotFallBack(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "solo.json"),
		[]byte(`{"id":"solo","title":"Solo","instructions":"You review."}`), 0o600))
	spec := "---\ntp:\n  review_roles:\n    solo:\n      enabled: false\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 2, code, "an emptied reviewer panel refuses instead of falling back")
	assert.NotContains(t, stdout, "implementer", "no fallback to the embedded default corpus")
	assert.NotContains(t, stdout, "solo", "the deactivated role is not emitted either")
}

// TestAudit_DeactivatingEveryUserRoleDoesNotFallBack: the audit half of test 11
// — the same placement, applied to the auditor panel.
func TestAudit_DeactivatingEveryUserRoleDoesNotFallBack(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "solo.json"),
		[]byte(`{"id":"solo","title":"Solo","instructions":"You audit."}`), 0o600))
	spec := "---\ntp:\n  audit_roles:\n    solo:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 2, code, "an emptied auditor panel refuses instead of falling back")
	assert.NotContains(t, stdout, "spec-coverage", "no fallback to the embedded default corpus")
	assert.NotContains(t, stdout, "solo", "the deactivated role is not emitted either")
}

// TestReview_DeactivatedRoleNamedDisabledBySpec: §2.4 (test 1) — a reviewer the
// spec deactivated with enabled: false is removed from the emitted panel AND
// named in skipped_roles with the new reason disabled-by-spec, so the drop is
// visible rather than silent. The round-1 regression skip is asserted alongside
// it: the new reason is added to the existing ones, not substituted for them.
func TestReview_DeactivatedRoleNamedDisabledBySpec(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "keeper.json", `{"id":"keeper","title":"Keeper","instructions":"You review.","focus":["q"]}`)
	writeReviewerRole(t, dir, "dropped.json", `{"id":"dropped","title":"Dropped","instructions":"You review.","focus":["q"]}`)
	spec := "---\ntp:\n  review_roles:\n    dropped:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "keeper", "the surviving reviewer still emits")
	assert.NotContains(t, byRole, "dropped", "the deactivated reviewer emits no prompt")

	byReason := map[string]string{}
	for _, s := range skippedRolesFrom(t, stdout) {
		byReason[s["role"].(string)] = s["reason"].(string)
	}
	assert.Equal(t, "disabled-by-spec", byReason["dropped"], "a deactivated reviewer is named, not silently absent")
	assert.Equal(t, "no-baseline", byReason["regression"], "the pre-existing reasons still apply alongside it")
}

// TestAudit_DeactivatedRoleNamedDisabledBySpec: the audit half of test 1. The
// two payloads assemble skipped_roles in different places — buildReviewPrompts
// for review, runAudit for audit — so each is pinned separately.
func TestAudit_DeactivatedRoleNamedDisabledBySpec(t *testing.T) {
	spec := "---\ntp:\n  audit_roles:\n    dropped:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"
	dir := writeAuditorCorpusProject(t, spec, "spec-coverage", "keeper", "dropped")

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := auditPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "keeper", "the surviving auditor still emits")
	assert.NotContains(t, byRole, "dropped", "the deactivated auditor emits no prompt")

	byReason := map[string]string{}
	for _, s := range skippedRolesFrom(t, stdout) {
		byReason[s["role"].(string)] = s["reason"].(string)
	}
	assert.Equal(t, "disabled-by-spec", byReason["dropped"], "a deactivated auditor is named, not silently absent")
}

// TestReview_DeactivatedRoleDropsItsFocus: §2.4 test 2 — `enabled: false` and
// `focus` on the SAME role deactivates it AND applies that focus nowhere in the
// output. The focus half is the discriminating one: an implementation that
// removes the role from the panel but still layers its override focus onto the
// survivors satisfies every panel assertion while leaking the question, so the
// assertion is over the whole payload, not just the dropped role's prompt.
func TestReview_DeactivatedRoleDropsItsFocus(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "keeper.json", `{"id":"keeper","title":"Keeper","instructions":"You review.","focus":["q"]}`)
	writeReviewerRole(t, dir, "dropped.json", `{"id":"dropped","title":"Dropped","instructions":"You review.","focus":["q"]}`)
	spec := "---\ntp:\n  review_roles:\n    dropped:\n      enabled: false\n      focus:\n        - \"DEACTIVATED FOCUS QUESTION\"\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "keeper", "the surviving reviewer still emits")
	assert.NotContains(t, byRole, "dropped", "enabled: false wins over the sibling focus key")
	assert.Equal(t, []string{"disabled-by-spec"}, skipReasonsFor(t, stdout, "dropped"),
		"the deactivated role is still named once, for the deactivation")

	for role, prompt := range byRole {
		assert.NotContains(t, prompt, "DEACTIVATED FOCUS QUESTION",
			"a deactivated role's focus must not reach role %s", role)
	}
	assert.NotContains(t, stdout, "DEACTIVATED FOCUS QUESTION",
		"a deactivated role's focus is applied nowhere in the output")
}

// TestAudit_DeactivatedRoleDropsItsFocus: the audit half of test 2. The two
// phases resolve their overrides through the same ResolveOverrideFocus but
// assemble their payloads separately, so each is pinned.
func TestAudit_DeactivatedRoleDropsItsFocus(t *testing.T) {
	spec := "---\ntp:\n  audit_roles:\n    dropped:\n      enabled: false\n      focus:\n        - \"DEACTIVATED AUDIT FOCUS\"\n---\n# Spec\n## 1. Widgets\ncontent\n"
	dir := writeAuditorCorpusProject(t, spec, "spec-coverage", "keeper", "dropped")

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := auditPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "keeper", "the surviving auditor still emits")
	assert.NotContains(t, byRole, "dropped", "enabled: false wins over the sibling focus key")
	assert.Equal(t, []string{"disabled-by-spec"}, skipReasonsFor(t, stdout, "dropped"),
		"the deactivated auditor is still named once, for the deactivation")
	assert.NotContains(t, stdout, "DEACTIVATED AUDIT FOCUS",
		"a deactivated auditor's focus is applied nowhere in the output")
}

// TestReview_CompactOmitsDisabledBySpec: §2.4 test 16 — --compact omits
// skipped_roles, disabled-by-spec entries included, so a driver cannot tell a
// deactivated role from any other absence under compact.
//
// The same fixture is run twice on purpose. The non-compact run asserts the
// entry IS emitted; without it the compact assertion would pass trivially
// against an implementation that never produces a disabled-by-spec entry at all.
func TestReview_CompactOmitsDisabledBySpec(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "keeper.json", `{"id":"keeper","title":"Keeper","instructions":"You review.","focus":["q"]}`)
	writeReviewerRole(t, dir, "dropped.json", `{"id":"dropped","title":"Dropped","instructions":"You review.","focus":["q"]}`)
	spec := "---\ntp:\n  review_roles:\n    dropped:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.Equal(t, []string{"disabled-by-spec"}, skipReasonsFor(t, stdout, "dropped"),
		"the fixture really emits the entry that --compact must omit")

	stdout, stderr, code = runTP(t, dir, "review", "spec.md", "--compact")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	_, hasField := out["skipped_roles"]
	assert.False(t, hasField, "--compact omits skipped_roles, disabled-by-spec included")
	assert.NotContains(t, stdout, "disabled-by-spec", "no disabled-by-spec entry survives --compact")
}

// TestAudit_CompactOmitsDisabledBySpec: the audit half of test 16. audit
// assembles skipped_roles in runAudit, separately from review, so its --compact
// gate is pinned separately too.
func TestAudit_CompactOmitsDisabledBySpec(t *testing.T) {
	spec := "---\ntp:\n  audit_roles:\n    dropped:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"
	dir := writeAuditorCorpusProject(t, spec, "spec-coverage", "keeper", "dropped")

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.Equal(t, []string{"disabled-by-spec"}, skipReasonsFor(t, stdout, "dropped"),
		"the fixture really emits the entry that --compact must omit")

	stdout, stderr, code = runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go", "--compact")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	_, hasField := out["skipped_roles"]
	assert.False(t, hasField, "--compact omits skipped_roles, disabled-by-spec included")
	assert.NotContains(t, stdout, "disabled-by-spec", "no disabled-by-spec entry survives --compact")
}

// TestReview_EnabledFalseOnlyEntrySuppressesLensShim: §2.3 test 12 — a spec
// that uses review_roles only to deactivate a role suppresses the legacy
// tp: lens shim, exactly as one that adds focus does.
//
// The lens question is the discriminating assertion. ResolveOverrideFocus only
// reaches TranslateLegacyLens when fm.ReviewRoles is empty, so an enabled-only
// entry that never landed in that map would take the shim branch and fan
// LEGACY LENS QUESTION out to every surviving reviewer while every panel
// assertion below still held.
func TestReview_EnabledFalseOnlyEntrySuppressesLensShim(t *testing.T) {
	dir := t.TempDir()
	spec := "---\ntp:\n  review_roles:\n    tester:\n      enabled: false\n  lens:\n    all:\n      - \"LEGACY LENS QUESTION\"\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "implementer", "the surviving reviewers still emit")
	assert.Contains(t, byRole, "architect")
	assert.NotContains(t, byRole, "tester", "the deactivated reviewer emits no prompt")
	assert.Equal(t, []string{"disabled-by-spec"}, skipReasonsFor(t, stdout, "tester"),
		"the deactivation itself still took effect")

	for role, prompt := range byRole {
		assert.NotContains(t, prompt, "LEGACY LENS QUESTION",
			"the lens shim is suppressed; it must not reach role %s", role)
	}
	assert.NotContains(t, stdout, "LEGACY LENS QUESTION",
		"a deactivation-only review_roles block suppresses the lens shim entirely")
}

// refusalMessage decodes tp's JSON error object off stderr so the empty-phase
// message and hint can be asserted verbatim rather than by substring.
func refusalMessage(t *testing.T, stderr string) (msg, hint string) {
	t.Helper()
	var e struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &e), "stderr: %s", stderr)
	return e.Error, e.Hint
}

// TestReview_EmptyPhaseMessageRendersSortedIDs: deactivating every reviewer
// exits 2 with §2.5's empty-phase message verbatim on stderr — the phase word
// rendered from engine.PhaseReviewers and the deactivated ids sorted and
// comma-separated — plus the fixed hint, and an empty stdout (test 7).
func TestReview_EmptyPhaseMessageRendersSortedIDs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	// Written in reverse order so a rendering that echoed corpus order rather
	// than sorting would produce "tester, architect".
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "tester.json"),
		[]byte(`{"id":"tester","title":"Tester","instructions":"You test."}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "architect.json"),
		[]byte(`{"id":"architect","title":"Architect","instructions":"You architect."}`), 0o600))
	spec := "---\ntp:\n  review_roles:\n    tester:\n      enabled: false\n    architect:\n      enabled: false\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Empty(t, stdout, "no prompt is emitted before the refusal")
	msg, hint := refusalMessage(t, stderr)
	assert.Equal(t, "every reviewers role is deactivated by this spec: architect, tester", msg)
	assert.Equal(t, "re-enable at least one role, or remove the enabled: false entries", hint)
}

// TestAudit_EmptyPhaseMessageRendersSortedIDs: the audit half of test 7 — the
// same message with the phase word rendered from engine.PhaseAuditors.
func TestAudit_EmptyPhaseMessageRendersSortedIDs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "zeta.json"),
		[]byte(`{"id":"zeta","title":"Zeta","instructions":"You audit."}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "alpha.json"),
		[]byte(`{"id":"alpha","title":"Alpha","instructions":"You audit."}`), 0o600))
	spec := "---\ntp:\n  audit_roles:\n    zeta:\n      enabled: false\n    alpha:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Empty(t, stdout, "no prompt is emitted before the refusal")
	msg, hint := refusalMessage(t, stderr)
	assert.Equal(t, "every auditors role is deactivated by this spec: alpha, zeta", msg)
	assert.Equal(t, "re-enable at least one role, or remove the enabled: false entries", hint)
}

// TestReview_EmptyPhaseNamesOnlySpecDeactivatedIDs: the empty-phase list carries
// only the ids THIS spec deactivated with enabled: false. An id already absent
// for another reason is excluded — "aardvark" is filtered out by domains before
// the drop set is computed, and "ghost" has no role file at all; both take
// §2.3's "matches no active role" path and contribute no drop. The phase is
// emptied partly by domains and partly by enabled: false, and the message names
// only the latter (§2.5). "aardvark" sorts first, so an implementation keyed on
// the frontmatter entries rather than the drop set would lead the list with it.
func TestReview_EmptyPhaseNamesOnlySpecDeactivatedIDs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "aardvark.json"),
		[]byte(`{"id":"aardvark","title":"Aardvark","instructions":"You review prose.","domains":["prose"]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "beta.json"),
		[]byte(`{"id":"beta","title":"Beta","instructions":"You review."}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "delta.json"),
		[]byte(`{"id":"delta","title":"Delta","instructions":"You review."}`), 0o600))
	// The spec carries no tp.domain, so it is a software spec and "aardvark"
	// (prose-only) is not active; "ghost" names no role file at all.
	spec := "---\ntp:\n  review_roles:\n    aardvark:\n      enabled: false\n    beta:\n      enabled: false\n    delta:\n      enabled: false\n    ghost:\n      enabled: false\n---\n# Spec\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Empty(t, stdout, "no prompt is emitted before the refusal")
	msg, hint := refusalMessage(t, stderr)
	assert.Equal(t, "every reviewers role is deactivated by this spec: beta, delta", msg,
		"only the ids this spec deactivated, sorted and comma-separated")
	assert.Equal(t, "re-enable at least one role, or remove the enabled: false entries", hint)
}

// TestAudit_EmptyPhaseNamesOnlySpecDeactivatedIDs: the audit half of the same
// rule — the auditor phase emptied partly by domains and partly by enabled:
// false names only the deactivated ids (§2.5).
func TestAudit_EmptyPhaseNamesOnlySpecDeactivatedIDs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "aardvark.json"),
		[]byte(`{"id":"aardvark","title":"Aardvark","instructions":"You audit prose.","domains":["prose"]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "beta.json"),
		[]byte(`{"id":"beta","title":"Beta","instructions":"You audit."}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "delta.json"),
		[]byte(`{"id":"delta","title":"Delta","instructions":"You audit."}`), 0o600))
	spec := "---\ntp:\n  audit_roles:\n    aardvark:\n      enabled: false\n    beta:\n      enabled: false\n    delta:\n      enabled: false\n    ghost:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Empty(t, stdout, "no prompt is emitted before the refusal")
	msg, hint := refusalMessage(t, stderr)
	assert.Equal(t, "every auditors role is deactivated by this spec: beta, delta", msg,
		"only the ids this spec deactivated, sorted and comma-separated")
	assert.Equal(t, "re-enable at least one role, or remove the enabled: false entries", hint)
}

// TestAudit_SpecCoverageCannotBeDeactivated: an auditor drop set containing
// spec-coverage exits 2 with §2.5's second refusal verbatim — message and hint
// — even though another auditor remains active. It is not an emptiness check:
// routeChecklist routes every spec-derived item to spec-coverage alone, so
// deactivating it drops the whole spec-derived checklist while "keeper" would
// still have emitted.
func TestAudit_SpecCoverageCannotBeDeactivated(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "spec-coverage.json"),
		[]byte(`{"id":"spec-coverage","title":"Spec Coverage","instructions":"You audit coverage."}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(audDir, "keeper.json"),
		[]byte(`{"id":"keeper","title":"Keeper","instructions":"You audit."}`), 0o600))
	spec := "---\ntp:\n  audit_roles:\n    spec-coverage:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Empty(t, stdout, "no prompt is emitted before the refusal")
	msg, hint := refusalMessage(t, stderr)
	assert.Equal(t, "spec-coverage cannot be deactivated: it carries the entire spec-derived checklist", msg)
	assert.Equal(t, "remove the enabled: false entry for spec-coverage", hint)
}

// specCoverageDeactivatedSpec is the single frontmatter BOTH halves of the
// drop-set test share: tp.audit_roles naming spec-coverage with enabled: false.
// The halves differ only in the auditor corpus on disk — that difference alone
// is what discriminates a refusal keyed on §2.3's drop set from one keyed on the
// frontmatter entry (§2.5, test 10).
const specCoverageDeactivatedSpec = "---\ntp:\n  audit_roles:\n    spec-coverage:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"

// writeAuditorCorpusProject lays out a project whose .tp/auditors corpus holds
// exactly the given role ids, plus the given spec and one code file to audit.
func writeAuditorCorpusProject(t *testing.T, spec string, auditorIDs ...string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	for _, id := range auditorIDs {
		require.NoError(t, os.WriteFile(filepath.Join(audDir, id+".json"),
			[]byte(`{"id":"`+id+`","title":"`+id+`","instructions":"You audit."}`), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
	return dir
}

// TestAudit_SpecCoverageRefusalKeysOnDropSet: the refusal keys on the drop set,
// not on the frontmatter entry (§2.5, test 10). Both halves run the identical
// specCoverageDeactivatedSpec and differ only in the auditor corpus: with
// spec-coverage.json on disk the role is active, lands in the drop set, and tp
// exits 2; without it the same entry matches no active role, contributes no
// drop, and the run proceeds. An implementation that scanned fm.AuditRoles for
// an enabled: false spec-coverage entry would exit 2 in both halves and fail the
// second — that failure mode is the point of this test.
func TestAudit_SpecCoverageRefusalKeysOnDropSet(t *testing.T) {
	t.Run("corpus holds spec-coverage", func(t *testing.T) {
		dir := writeAuditorCorpusProject(t, specCoverageDeactivatedSpec, "spec-coverage", "keeper")

		stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		require.Equal(t, 2, code, "an active spec-coverage in the drop set refuses; stderr: %s", stderr)
		assert.Empty(t, stdout, "no prompt is emitted before the refusal")
		msg, hint := refusalMessage(t, stderr)
		assert.Equal(t, "spec-coverage cannot be deactivated: it carries the entire spec-derived checklist", msg)
		assert.Equal(t, "remove the enabled: false entry for spec-coverage", hint)
	})

	t.Run("populated corpus lacks spec-coverage", func(t *testing.T) {
		// The corpus must be POPULATED rather than empty: an empty .tp/auditors
		// falls back to the embedded default corpus, which does hold
		// spec-coverage, so the drop would be re-armed and the half would
		// measure the fallback instead of the key.
		dir := writeAuditorCorpusProject(t, specCoverageDeactivatedSpec, "keeper", "second")

		stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		require.Equal(t, 0, code, "the identical frontmatter must not refuse when no spec-coverage role is active; stderr: %s", stderr)
		byRole := auditPromptsByRole(t, stdout)
		assert.Contains(t, byRole, "keeper", "the run proceeds over the corpus that is present")
		assert.Contains(t, byRole, "second")
		assert.NotContains(t, byRole, "spec-coverage", "the entry named no active role, so nothing was dropped or emitted")
		// The entry takes §2.3's "matches no active role" warning path; the
		// warning text is asserted by the engine test
		// TestResolveOverrideFocus_SpecCoverageDropDependsOnCorpus, because
		// output.Info is suppressed in the JSON mode every runTP call uses.
	})
}

// TestReview_SpecCoverageEntryTakesWarningPath: a tp.review_roles entry naming
// spec-coverage ALWAYS takes the "matches no active role" path — the review
// caller cannot trip the auditor refusal (§2.5, test 10). The auditor corpus on
// disk holds spec-coverage.json, the exact state that makes tp audit refuse, yet
// review resolves overrides against the reviewer panel, where spec-coverage is
// not an active role, so it contributes no drop and the run proceeds.
func TestReview_SpecCoverageEntryTakesWarningPath(t *testing.T) {
	dir := writeAuditorCorpusProject(t,
		"---\ntp:\n  review_roles:\n    spec-coverage:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n",
		"spec-coverage", "keeper")
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "solo.json"),
		[]byte(`{"id":"solo","title":"Solo","instructions":"You review."}`), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "the review caller never trips the spec-coverage refusal; stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "solo", "the reviewer panel emits normally")
	assert.NotContains(t, byRole, "spec-coverage", "the entry matched no active reviewer role")
}

// TestAudit_SpecCoverageRefusalPrecedesEmptyPhase: a spec that trips BOTH §2.5
// refusals at once reports the spec-coverage one, per §2.6's decided order
// (domain filtering, the unknown-id check, the spec-coverage refusal, the
// empty-phase refusal). The fixture deactivates spec-coverage AND every other
// active auditor, so the drop set both contains spec-coverage and empties the
// panel; the second sub-test is the control that the empty-phase condition is
// really armed by that shape — the identical "deactivate every auditor" spec
// over a corpus without spec-coverage produces the empty-phase message. So in
// the combined case it is the ordering, not a single condition holding, that
// selects which message the agent reads.
func TestAudit_SpecCoverageRefusalPrecedesEmptyPhase(t *testing.T) {
	t.Run("both refusals hold", func(t *testing.T) {
		dir := writeAuditorCorpusProject(t,
			"---\ntp:\n  audit_roles:\n    keeper:\n      enabled: false\n    spec-coverage:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n",
			"spec-coverage", "keeper")

		stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		assert.Empty(t, stdout, "no prompt is emitted before the refusal")
		msg, hint := refusalMessage(t, stderr)
		assert.Equal(t, "spec-coverage cannot be deactivated: it carries the entire spec-derived checklist", msg,
			"the spec-coverage refusal is decided before the empty-phase one")
		assert.Equal(t, "remove the enabled: false entry for spec-coverage", hint)
		assert.NotContains(t, stderr, "every auditors role is deactivated by this spec",
			"the empty-phase message never reaches the agent")
	})

	t.Run("empty phase alone", func(t *testing.T) {
		dir := writeAuditorCorpusProject(t,
			"---\ntp:\n  audit_roles:\n    keeper:\n      enabled: false\n    second:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n",
			"keeper", "second")

		_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		msg, _ := refusalMessage(t, stderr)
		assert.Equal(t, "every auditors role is deactivated by this spec: keeper, second", msg,
			"without spec-coverage in the drop set the same shape reports the empty-phase refusal")
	})
}

// The two §2.5 refusal messages, pinned as literals so the emission-only scope
// is asserted by their ABSENCE rather than by a non-2 exit code — a malformed
// argument also exits 2, so an exit-code assertion would not discriminate. The
// empty-phase message renders the phase word, so both renderings are listed.
const (
	refusalEmptyPhaseReviewers = "every reviewers role is deactivated by this spec"
	refusalEmptyPhaseAuditors  = "every auditors role is deactivated by this spec"
	refusalSpecCoverage        = "spec-coverage cannot be deactivated: it carries the entire spec-derived checklist"
)

// assertNoRefusalMessage fails when either §2.5 refusal reached the agent on
// stdout or stderr.
func assertNoRefusalMessage(t *testing.T, stdout, stderr string) {
	t.Helper()
	for _, msg := range []string{refusalEmptyPhaseReviewers, refusalEmptyPhaseAuditors, refusalSpecCoverage} {
		assert.NotContains(t, stdout, msg, "a §2.5 refusal reached stdout outside prompt emission")
		assert.NotContains(t, stderr, msg, "a §2.5 refusal reached stderr outside prompt emission")
	}
}

// bothPhasesDeactivatedSpec arms BOTH §2.5 refusals from one spec: it
// deactivates the only reviewer (emptying the reviewer phase) and every
// auditor including spec-coverage (tripping the spec-coverage refusal, which
// §2.6 decides first).
const bothPhasesDeactivatedSpec = "---\ntp:\n  review_roles:\n    solo:\n      enabled: false\n  audit_roles:\n    spec-coverage:\n      enabled: false\n    keeper:\n      enabled: false\n---\n# Spec\n## 1. Widgets\ncontent\n"

// writeBothPhasesDeactivatedProject lays out a project whose reviewer and
// auditor corpora are both fully deactivated by bothPhasesDeactivatedSpec, plus
// the code file the audit emission path needs and an empty findings NDJSON for
// the modes that take one.
func writeBothPhasesDeactivatedProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "solo.json"),
		[]byte(`{"id":"solo","title":"Solo","instructions":"You review."}`), 0o600))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o755))
	for _, id := range []string{"spec-coverage", "keeper"} {
		require.NoError(t, os.WriteFile(filepath.Join(audDir, id+".json"),
			[]byte(`{"id":"`+id+`","title":"`+id+`","instructions":"You audit."}`), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(bothPhasesDeactivatedSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "findings.ndjson"), nil, 0o600))
	return dir
}

// TestRefusals_FireOnlyOnPromptEmission: both §2.5 refusals fire only in each
// command's prompt-emission mode — the default invocation with a spec
// positional and no mode flag (§2.5 item 1, test 8). The armed sub-test is the
// control: it proves this exact spec and corpus DO trip both refusals, so a
// mode sub-test that stays silent is measuring the mode scope and not a
// toothless fixture.
//
// `--merge`, `--resolve`, `--resolve-all` and `--report` take an NDJSON
// positional and never parse the spec, so they are unaffected by construction
// and are not asserted; `tp audit` registers none of them. `tp review
// --perspective` does take the spec but short-circuits before the corpus is
// resolved, and is asserted separately in
// TestRefusals_PerspectiveShortCircuitsBeforeCorpus.
func TestRefusals_FireOnlyOnPromptEmission(t *testing.T) {
	t.Run("control: prompt emission refuses", func(t *testing.T) {
		dir := writeBothPhasesDeactivatedProject(t)
		_, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		assert.Contains(t, stderr, refusalEmptyPhaseReviewers, "the reviewer phase is armed")

		dir = writeBothPhasesDeactivatedProject(t)
		_, stderr, code = runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		assert.Contains(t, stderr, refusalSpecCoverage, "the auditor phase is armed")
	})

	// wantExit follows §2.5 item 1: --record and --verify exit 0, --status
	// exits 0, and --status --check exits 0 or 1 by convergence.
	for _, tc := range []struct {
		name     string
		args     []string
		wantExit []int
	}{
		{"review --record", []string{"review", "spec.md", "--record", "findings.ndjson"}, []int{0}},
		{"review --status", []string{"review", "spec.md", "--status"}, []int{0}},
		{"review --status --check", []string{"review", "spec.md", "--status", "--check"}, []int{0, 1}},
		{"review --verify", []string{"review", "spec.md", "--verify", "--findings", "findings.ndjson"}, []int{0}},
		{"audit --record", []string{"audit", "spec.md", "--record", "findings.ndjson"}, []int{0}},
		{"audit --status", []string{"audit", "spec.md", "--status"}, []int{0}},
		{"audit --status --check", []string{"audit", "spec.md", "--status", "--check"}, []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeBothPhasesDeactivatedProject(t)
			stdout, stderr, code := runTP(t, dir, tc.args...)
			assert.Contains(t, tc.wantExit, code, "stderr: %s", stderr)
			assertNoRefusalMessage(t, stdout, stderr)
		})
	}
}

// TestRefusals_PerspectiveShortCircuitsBeforeCorpus: `tp review --perspective`
// is the one non-default mode that takes the spec as its positional AND
// genuinely parses it, so §2.5's emission-only scope has to be asserted for it
// rather than left to construction (test 8). Every perspective dispatches to
// its single fixed prompt inside runReview before resolveRolePanel is reached,
// so the spec that arms both refusals still emits that prompt and carries
// neither message. Routing the panel resolution ahead of the perspective
// dispatch is precisely the future refactor this pins: it would make these runs
// start refusing.
//
// The control sub-test re-proves the fixture is armed, so a silent perspective
// run measures the short-circuit and not a toothless spec.
func TestRefusals_PerspectiveShortCircuitsBeforeCorpus(t *testing.T) {
	t.Run("control: prompt emission refuses", func(t *testing.T) {
		dir := writeBothPhasesDeactivatedProject(t)
		_, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		require.Contains(t, stderr, refusalEmptyPhaseReviewers, "the fixture really refuses")
	})

	// Every value --perspective accepts. regression takes its stateless mode
	// (b) form (--diff-from plus --findings) so the assertion stays about the
	// corpus and not about a missing state directory.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"documentation", []string{"review", "spec.md", "--perspective", "documentation", "--docs-path", "docs"}},
		{"testing", []string{"review", "spec.md", "--perspective", "testing", "--test-path", "tests"}},
		{"code-audit", []string{"review", "spec.md", "--perspective", "code-audit", "--affected-files", "code.go"}},
		{"regression", []string{"review", "spec.md", "--perspective", "regression", "--diff-from", "baseline.md", "--findings", "findings.ndjson"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeBothPhasesDeactivatedProject(t)
			require.NoError(t, os.Mkdir(filepath.Join(dir, "docs"), 0o755))
			require.NoError(t, os.Mkdir(filepath.Join(dir, "tests"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "baseline.md"),
				[]byte("# Spec\n## 1. Widgets\nolder content\n"), 0o600))

			stdout, stderr, code := runTP(t, dir, tc.args...)
			require.Equal(t, 0, code, "stderr: %s", stderr)
			assert.NotEmpty(t, stdout, "the perspective emitted its single fixed prompt")
			assertNoRefusalMessage(t, stdout, stderr)
		})
	}
}

// TestRefusals_WriteNothingBeforeRefusing: §2.5 item 2 — corpus and override
// resolution run ahead of every write the emission path performs, so a refused
// run leaves no state behind. Asserted per command over that command's OWN
// artifacts, because the two artifact sets differ: tp review's emission path
// calls EnsureReviewState (creating .tp-review/<spec>/ and state.json) before
// WriteSnapshotAtomic, so all three must be absent; tp audit only snapshots, so
// its artifact is the round snapshot.
//
// The review half deliberately runs WITHOUT --no-state — that is the only mode
// that arms the state lifecycle, and it is the mode every other empty-phase
// test avoids.
func TestRefusals_WriteNothingBeforeRefusing(t *testing.T) {
	t.Run("review", func(t *testing.T) {
		dir := writeBothPhasesDeactivatedProject(t)

		stdout, stderr, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, refusalEmptyPhaseReviewers, "the fixture really refuses")

		stateDir := filepath.Join(dir, ".tp-review", "spec")
		assert.NoDirExists(t, stateDir, "the refusal precedes EnsureReviewState")
		assert.NoFileExists(t, filepath.Join(stateDir, "state.json"))
		assert.NoFileExists(t, filepath.Join(stateDir, "snapshot-round-1.md"))
	})

	t.Run("audit", func(t *testing.T) {
		dir := writeBothPhasesDeactivatedProject(t)

		stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		require.Equal(t, 2, code, "stderr: %s", stderr)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, refusalSpecCoverage, "the fixture really refuses")

		assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "snapshot-audit-round-1.md"),
			"the refusal precedes the round snapshot")
	})
}

// skipReasonsFor returns every reason skipped_roles records for one role id, so
// a role reported twice is distinguishable from one reported once.
func skipReasonsFor(t *testing.T, stdout, role string) []string {
	t.Helper()
	reasons := make([]string, 0)
	for _, s := range skippedRolesFrom(t, stdout) {
		if s["role"] == role {
			reasons = append(reasons, s["reason"].(string))
		}
	}
	return reasons
}

// TestReview_EnabledFalseUnknownIDChangesNothing: §2.3 test 6, first clause —
// enabled: false on an id no corpus holds at all takes the "matches no active
// role" path and changes nothing: every default reviewer still emits and the id
// is named in no skipped_roles entry, because an id outside the active panel is
// not a drop. The warning text is asserted by the engine test
// TestResolveOverrideFocus_OutsideActivePanelWarnsAndDropsNothing, since
// output.Info is silent in the JSON mode every runTP call uses.
func TestReview_EnabledFalseUnknownIDChangesNothing(t *testing.T) {
	dir := t.TempDir()
	spec := "---\ntp:\n  review_roles:\n    ghost:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := reviewPromptsByRole(t, stdout)
	for _, role := range []string{"implementer", "tester", "architect"} {
		assert.Contains(t, byRole, role, "the panel is untouched by an entry matching no active role")
	}
	assert.Empty(t, skipReasonsFor(t, stdout, "ghost"), "an id outside the active panel is never named as skipped")
	for _, s := range skippedRolesFrom(t, stdout) {
		assert.NotEqual(t, "disabled-by-spec", s["reason"], "nothing was deactivated: %v", s)
	}
}

// writeDomainFilteredCorpusProject lays out a software-domain spec over a
// reviewer corpus holding one role that applies to every domain and one
// prose-only role that domain filtering removes before override resolution.
func writeDomainFilteredCorpusProject(t *testing.T, spec string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "sw-role.json",
		`{"id":"sw-role","title":"SW","instructions":"You review.","focus":["q"]}`)
	writeReviewerRole(t, dir, "prose-role.json",
		`{"id":"prose-role","title":"Prose","instructions":"You review.","focus":["q"],"domains":["prose"]}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	return dir
}

// TestReview_DomainFilteredRoleTakesWarningPath: §2.3 test 6's second clause and
// test 13 (Non-Goal 3). A role the corpus holds but domains removed is not in
// the active set, so an enabled entry naming it takes the same "matches no
// active role" path as an id in no corpus at all.
//
// The discriminating assertion is that the role appears in skipped_roles EXACTLY
// ONCE and with domain-mismatch: an implementation that added the id to §2.3's
// drop set before checking the active set would report it a second time, with
// disabled-by-spec, since the two skip lists are appended independently.
func TestReview_DomainFilteredRoleTakesWarningPath(t *testing.T) {
	t.Run("enabled false", func(t *testing.T) {
		dir := writeDomainFilteredCorpusProject(t,
			"---\ntp:\n  review_roles:\n    prose-role:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent\n")

		stdout, stderr, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code, "stderr: %s", stderr)

		byRole := reviewPromptsByRole(t, stdout)
		assert.Contains(t, byRole, "sw-role", "the surviving reviewer still emits")
		assert.NotContains(t, byRole, "prose-role", "domain filtering had already removed it")
		assert.Equal(t, []string{"domain-mismatch"}, skipReasonsFor(t, stdout, "prose-role"),
			"named once, for the domain — never a second time as disabled-by-spec")
	})

	t.Run("enabled true", func(t *testing.T) {
		// Test 13: enabled: true does not resurrect a role domains removed —
		// the entry takes the same warning path, the role stays absent, and its
		// override focus reaches no prompt.
		dir := writeDomainFilteredCorpusProject(t,
			"---\ntp:\n  review_roles:\n    prose-role:\n      enabled: true\n      focus:\n        - \"RESURRECTION FOCUS\"\n---\n# Spec\n## 1. A\ncontent\n")

		stdout, stderr, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code, "stderr: %s", stderr)

		byRole := reviewPromptsByRole(t, stdout)
		assert.NotContains(t, byRole, "prose-role", "enabled: true resurrects no domain-filtered role")
		require.Contains(t, byRole, "sw-role")
		for role, prompt := range byRole {
			assert.NotContains(t, prompt, "RESURRECTION FOCUS", "the override focus must not reach role %s", role)
		}
		assert.Equal(t, []string{"domain-mismatch"}, skipReasonsFor(t, stdout, "prose-role"),
			"the role stays reported as domain-filtered, once")
	})
}

// TestReview_RegressionDoesNotSatisfyTheEmptinessCheck is test 9's second half.
// The built-in regression role is convergence machinery appended to emission
// separately, never a corpus reviewer (Non-Goal 4), so it does not count as an
// active reviewer for §2.5's emptiness check: a spec deactivating every corpus
// reviewer still exits 2 even in a round where regression would emit.
//
// The fixture runs WITH the state directory, at round 2 after a recorded round 1
// over an edited spec — the exact condition under which review appends the
// regression prompt, asserted here as a control before the deactivation is
// added. Under --no-state there is no baseline snapshot, regression is never a
// candidate, and the refusal would hold for an implementation that did count it.
func TestReview_RegressionDoesNotSatisfyTheEmptinessCheck(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "solo.json",
		`{"id":"solo","title":"Solo","instructions":"You review."}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. A\noriginal\n"), 0o600))

	// Round 1 emits and is recorded, so round 2 has a baseline snapshot.
	_, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = recordRound(t, dir, "")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	// Control: with the body edited and nothing deactivated, round 2 does
	// append the regression prompt — so the refusal below is asserted against
	// a live regression role, not an absent one.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. A\nrewritten\n"), 0o600))
	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.Contains(t, reviewPromptsByRole(t, stdout), "regression",
		"the fixture must be a round in which regression emits")

	// The same round, with the only corpus reviewer deactivated: the phase is
	// empty and tp refuses, because regression is no substitute for it.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("---\ntp:\n  review_roles:\n    solo:\n      enabled: false\n---\n# Spec\n## 1. A\nrewritten\n"), 0o600))
	stdout, stderr, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 2, code, "regression does not keep the reviewer phase non-empty; stderr: %s", stderr)
	assert.Empty(t, stdout, "no prompt is emitted — the regression one included")
	msg, hint := refusalMessage(t, stderr)
	assert.Equal(t, "every reviewers role is deactivated by this spec: solo", msg,
		"the message names only the corpus reviewer, never regression")
	assert.Equal(t, "re-enable at least one role, or remove the enabled: false entries", hint)
}

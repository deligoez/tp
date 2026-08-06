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
// resolved, and is outside this test's scope.
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

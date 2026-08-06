package engine

import (
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
)

func roleFocusByID(roles []model.Role) map[string][]string {
	m := make(map[string][]string)
	for i := range roles {
		m[roles[i].ID] = roles[i].Focus
	}
	return m
}

// TestResolveOverrideFocus_Additive appends a review override's focus to the
// matching corpus role's focus, project focus first (§10.2, §10.3).
func TestResolveOverrideFocus_Additive(t *testing.T) {
	roles := []model.Role{
		{ID: "implementer", Focus: []string{"corpus q1"}},
		{ID: "tester", Focus: []string{"corpus q2"}},
	}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    implementer:\n      focus:\n        - \"override q\"\n---\n"))

	out, warnings, disabled := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	assert.Empty(t, warnings)
	assert.Empty(t, disabled, "an override without enabled: false drops nothing")
	byID := roleFocusByID(out)
	assert.Equal(t, []string{"corpus q1", "override q"}, byID["implementer"], "project focus then override focus")
	assert.Equal(t, []string{"corpus q2"}, byID["tester"], "untargeted role unchanged")
}

// TestResolveOverrideFocus_UnknownID warns and ignores an override whose id
// matches no active role — including an attempt to override regression, which is
// never an active corpus role (§10.2, §5.2).
func TestResolveOverrideFocus_UnknownID(t *testing.T) {
	roles := []model.Role{{ID: "implementer", Focus: []string{"q"}}}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    ghost:\n      focus:\n        - \"x\"\n    regression:\n      focus:\n        - \"y\"\n---\n"))

	out, warnings, disabled := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	assert.Equal(t, []string{"q"}, roleFocusByID(out)["implementer"], "unknown overrides do not touch active roles")
	assert.Empty(t, disabled, "unknown overrides drop nothing")
	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, `override for "ghost" matches no active reviewers role`)
	assert.Contains(t, joined, `override for "regression" matches no active reviewers role`, "regression accepts no overrides")
}

// TestResolveOverrideFocus_LegacyLens applies the legacy tp: lens shim when no
// new overrides are present (§10.4).
func TestResolveOverrideFocus_LegacyLens(t *testing.T) {
	roles := []model.Role{
		{ID: "implementer", Focus: []string{"corpus"}},
		{ID: "tester", Focus: nil},
	}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  lens:\n    all:\n      - \"lens-all q\"\n    implementer:\n      - \"lens-role q\"\n---\n"))

	out, warnings, disabled := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	byID := roleFocusByID(out)
	assert.Equal(t, []string{"corpus", "lens-all q", "lens-role q"}, byID["implementer"])
	assert.Equal(t, []string{"lens-all q"}, byID["tester"], "lens.all fans out to every review role")
	assert.Contains(t, strings.Join(warnings, "\n"), "deprecated")
	assert.Empty(t, disabled, "the legacy lens carries no enabled key")
}

// TestResolveOverrideFocus_AuditPhase applies tp.audit_roles to audit roles and
// never applies the legacy review lens to auditors (§10.4).
func TestResolveOverrideFocus_AuditPhase(t *testing.T) {
	roles := []model.Role{{ID: "security", Focus: []string{"corpus"}}}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  audit_roles:\n    security:\n      focus:\n        - \"audit override\"\n  lens:\n    all:\n      - \"should not reach auditors\"\n---\n"))

	out, warnings, disabled := ResolveOverrideFocus(roles, fm, PhaseAuditors)
	assert.Equal(t, []string{"corpus", "audit override"}, roleFocusByID(out)["security"])
	assert.NotContains(t, strings.Join(warnings, "\n"), "deprecated", "the legacy lens shim never runs for audit")
	assert.Empty(t, disabled, "a focus-only audit override drops nothing")
}

// TestResolveOverrideFocus_EnabledFalseDropSet returns the sorted ids of the
// active roles this spec deactivated with enabled: false, while enabled: true
// and an id matching no active role contribute nothing; applying the drop is the
// caller's job, so the effective roles are untouched here (§2.3).
func TestResolveOverrideFocus_EnabledFalseDropSet(t *testing.T) {
	roles := []model.Role{
		{ID: "implementer", Focus: []string{"q"}},
		{ID: "tester", Focus: []string{"q"}},
		{ID: "architect", Focus: []string{"q"}},
	}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    tester:\n      enabled: false\n    architect:\n      enabled: false\n    implementer:\n      enabled: true\n    ghost:\n      enabled: false\n---\n"))

	out, _, disabled := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	assert.Equal(t, []string{"architect", "tester"}, disabled, "sorted ids of the active roles this spec deactivated")
	assert.Len(t, out, 3, "resolution reports the drop; the caller applies it")
}

// TestResolveOverrideFocus_SpecCoverageDropDependsOnCorpus: the value tp audit's
// spec-coverage refusal keys on — the drop set — depends on the corpus, not on
// the frontmatter entry. One and the same tp.audit_roles entry yields a
// spec-coverage drop against a panel holding that role and no drop at all
// against a populated panel lacking it, where it takes the "matches no active
// role" warning path instead; the review phase resolves against the reviewer
// panel and so never produces the drop (§2.3, §2.5). The CLI halves in
// TestAudit_SpecCoverageRefusalKeysOnDropSet assert the exit codes; the warning
// text is asserted here because tp suppresses it in JSON mode.
func TestResolveOverrideFocus_SpecCoverageDropDependsOnCorpus(t *testing.T) {
	fmAudit := ParseFrontmatterBytes([]byte("---\ntp:\n  audit_roles:\n    spec-coverage:\n      enabled: false\n---\n"))

	_, warnings, disabled := ResolveOverrideFocus([]model.Role{{ID: "spec-coverage"}, {ID: "keeper"}}, fmAudit, PhaseAuditors)
	assert.Equal(t, []string{"spec-coverage"}, disabled, "an active spec-coverage lands in the drop set")
	assert.NotContains(t, strings.Join(warnings, "\n"), "matches no active", "a matched entry is not an unknown id")

	_, warnings, disabled = ResolveOverrideFocus([]model.Role{{ID: "keeper"}, {ID: "second"}}, fmAudit, PhaseAuditors)
	assert.Empty(t, disabled, "the identical entry drops nothing when the corpus holds no spec-coverage role")
	assert.Contains(t, strings.Join(warnings, "\n"),
		`tp.audit_roles override for "spec-coverage" matches no active auditors role; ignored`)

	fmReview := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    spec-coverage:\n      enabled: false\n---\n"))
	_, warnings, disabled = ResolveOverrideFocus([]model.Role{{ID: "solo"}}, fmReview, PhaseReviewers)
	assert.Empty(t, disabled, "a review_roles entry naming spec-coverage produces no drop")
	assert.Contains(t, strings.Join(warnings, "\n"),
		`tp.review_roles override for "spec-coverage" matches no active reviewers role; ignored`)
}

// TestResolveOverrideFocus_OutsideActivePanelWarnsAndDropsNothing: an enabled
// entry whose id is not in the phase's active panel takes the "matches no active
// role" warning path and contributes no drop (§2.3, tests 6 and 13). The panel
// handed to resolution is already domain-filtered, so the two ways an id can be
// outside it — no corpus holds the role at all, and domains removed it — are one
// and the same input here; the CLI half
// TestReview_DomainFilteredRoleTakesWarningPath asserts what distinguishes them
// downstream, namely that a domain-filtered role is reported once and with
// domain-mismatch. The warning text is asserted at this layer because it is
// produced here; the CLI test TestReview_NoActiveRoleWarningVisibleInJSONMode
// asserts that the CLI puts it on stderr even in JSON mode.
func TestResolveOverrideFocus_OutsideActivePanelWarnsAndDropsNothing(t *testing.T) {
	fmDisable := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    prose-role:\n      enabled: false\n---\n"))

	out, warnings, disabled := ResolveOverrideFocus([]model.Role{{ID: "sw-role", Focus: []string{"q"}}}, fmDisable, PhaseReviewers)
	assert.Equal(t, []string{"q"}, roleFocusByID(out)["sw-role"], "the entry changes nothing about the surviving role")
	assert.Empty(t, disabled, "an id outside the active panel contributes no drop")
	assert.Contains(t, strings.Join(warnings, "\n"),
		`tp.review_roles override for "prose-role" matches no active reviewers role; ignored`)

	// The identical frontmatter against a panel that DOES hold the role drops
	// it and warns not: what decides the path is the active panel, not the
	// entry.
	_, warnings, disabled = ResolveOverrideFocus([]model.Role{{ID: "sw-role"}, {ID: "prose-role"}}, fmDisable, PhaseReviewers)
	assert.Equal(t, []string{"prose-role"}, disabled, "an active role is dropped")
	assert.NotContains(t, strings.Join(warnings, "\n"), "matches no active", "a matched entry is not an unknown id")

	// Non-Goal 3 (test 13): enabled: true resurrects nothing. With the role
	// outside the panel the entry takes the same warning path, adds no role and
	// lands no focus.
	fmEnable := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    prose-role:\n      enabled: true\n      focus:\n        - \"RESURRECTION FOCUS\"\n---\n"))
	out, warnings, disabled = ResolveOverrideFocus([]model.Role{{ID: "sw-role", Focus: []string{"q"}}}, fmEnable, PhaseReviewers)
	assert.Len(t, out, 1, "enabled: true adds no role to the panel")
	assert.Equal(t, []string{"q"}, roleFocusByID(out)["sw-role"], "the override focus reaches no role")
	assert.Empty(t, disabled, "enabled: true is not a drop")
	assert.Contains(t, strings.Join(warnings, "\n"),
		`tp.review_roles override for "prose-role" matches no active reviewers role; ignored`)
}

// TestResolveOverrideFocus_RegressionEnabledFalseTakesWarningPath is test 9's
// first half. The built-in regression role is convergence machinery appended to
// emission separately, so it is in neither corpus and is never in the active
// panel handed to resolution. An enabled: false entry naming it therefore takes
// §2.3's "matches no active role" path in both phases and contributes no drop —
// the only reading consistent with Non-Goal 4, since a drop would let a spec
// switch off the round-over-round diff.
func TestResolveOverrideFocus_RegressionEnabledFalseTakesWarningPath(t *testing.T) {
	cases := []struct {
		phase, field string
		panel        []model.Role
	}{
		{PhaseReviewers, "review_roles", []model.Role{{ID: "implementer", Focus: []string{"q"}}}},
		{PhaseAuditors, "audit_roles", []model.Role{{ID: "spec-coverage", Focus: []string{"q"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			fm := ParseFrontmatterBytes([]byte("---\ntp:\n  " + tc.field + ":\n    " + RegressionRoleID + ":\n      enabled: false\n---\n"))

			out, warnings, disabled := ResolveOverrideFocus(tc.panel, fm, tc.phase)
			assert.Empty(t, disabled, "regression is in no corpus, so it is never a drop")
			assert.Len(t, out, 1, "the active panel is untouched")
			assert.Equal(t, []string{"q"}, roleFocusByID(out)[tc.panel[0].ID])
			assert.Contains(t, strings.Join(warnings, "\n"),
				`tp.`+tc.field+` override for "regression" matches no active `+tc.phase+` role; ignored`)
		})
	}
}

// TestResolveOverrideFocus_ReviewRolesWithLensWarns: a spec carrying BOTH the
// new tp.review_roles form and the retired tp: lens reports the lens as ignored
// with §10.4's documented single warning on the review path too. The new form
// still wins outright — no lens question reaches any role — but dropping the
// lens silently was an asymmetry: an audit_roles-only spec already earns this
// warning by falling through to the shim.
func TestResolveOverrideFocus_ReviewRolesWithLensWarns(t *testing.T) {
	roles := []model.Role{
		{ID: "implementer", Focus: []string{"corpus"}},
		{ID: "tester", Focus: []string{"corpus"}},
	}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  review_roles:\n    implementer:\n      focus:\n        - \"new-form q\"\n  lens:\n    all:\n      - \"lens-all q\"\n---\n"))

	out, warnings, disabled := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, "legacy tp: lens is ignored", "the lens is dropped loudly, not silently")
	assert.NotContains(t, joined, "deprecated", "the new form wins; the lens is never auto-translated")
	byID := roleFocusByID(out)
	assert.Equal(t, []string{"corpus", "new-form q"}, byID["implementer"], "only the new form layers")
	assert.Equal(t, []string{"corpus"}, byID["tester"], "lens.all reaches nobody")
	assert.Empty(t, disabled)
}

// TestResolveOverrideFocus_NilFrontmatter: the exported resolver treats a nil
// frontmatter as a spec carrying none, returning the panel untouched in both
// phases instead of panicking on the field reads. TranslateLegacyLens guards
// its own nil the same way.
func TestResolveOverrideFocus_NilFrontmatter(t *testing.T) {
	roles := []model.Role{{ID: "implementer", Focus: []string{"corpus"}}}
	for _, phase := range []string{PhaseReviewers, PhaseAuditors} {
		out, warnings, disabled := ResolveOverrideFocus(roles, nil, phase)
		assert.Equal(t, []string{"corpus"}, roleFocusByID(out)["implementer"], phase)
		assert.Empty(t, warnings, phase)
		assert.Empty(t, disabled, phase)
	}
}

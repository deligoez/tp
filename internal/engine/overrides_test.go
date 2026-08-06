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

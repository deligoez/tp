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

	out, warnings := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	assert.Empty(t, warnings)
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

	out, warnings := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	assert.Equal(t, []string{"q"}, roleFocusByID(out)["implementer"], "unknown overrides do not touch active roles")
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

	out, warnings := ResolveOverrideFocus(roles, fm, PhaseReviewers)
	byID := roleFocusByID(out)
	assert.Equal(t, []string{"corpus", "lens-all q", "lens-role q"}, byID["implementer"])
	assert.Equal(t, []string{"lens-all q"}, byID["tester"], "lens.all fans out to every review role")
	assert.Contains(t, strings.Join(warnings, "\n"), "deprecated")
}
// TestResolveOverrideFocus_AuditPhase applies tp.audit_roles to audit roles and
// never applies the legacy review lens to auditors (§10.4).
func TestResolveOverrideFocus_AuditPhase(t *testing.T) {
	roles := []model.Role{{ID: "security", Focus: []string{"corpus"}}}
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  audit_roles:\n    security:\n      focus:\n        - \"audit override\"\n  lens:\n    all:\n      - \"should not reach auditors\"\n---\n"))

	out, warnings := ResolveOverrideFocus(roles, fm, PhaseAuditors)
	assert.Equal(t, []string{"corpus", "audit override"}, roleFocusByID(out)["security"])
	assert.NotContains(t, strings.Join(warnings, "\n"), "deprecated", "the legacy lens shim never runs for audit")
}

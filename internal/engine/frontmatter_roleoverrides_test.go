package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseFrontmatter_RoleOverrides covers valid tp.review_roles / tp.audit_roles
// parsing (§10.2): each map is keyed by role id and each value's only permitted
// key is focus, a string array.
func TestParseFrontmatter_RoleOverrides(t *testing.T) {
	spec := `---
tp:
  domain: software
  review_roles:
    implementer:
      focus:
        - "Does the happy path handle the empty batch?"
        - "Is the flock released on every error path?"
    architect:
      focus: []
  audit_roles:
    security:
      focus:
        - "Any command injection in the detector cmd?"
---
# Heading
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)
	assert.Empty(t, fm.Errors)
	assert.Empty(t, fm.Warnings, "a well-formed override set produces no warnings")

	assert.Equal(t, []string{
		"Does the happy path handle the empty batch?",
		"Is the flock released on every error path?",
	}, fm.ReviewRoles["implementer"].Focus)
	assert.Equal(t, []string{}, fm.ReviewRoles["architect"].Focus, "an empty focus list is retained as empty, not dropped")
	assert.Equal(t, []string{"Any command injection in the detector cmd?"}, fm.AuditRoles["security"].Focus)

	// Overrides never bleed across phases.
	assert.NotContains(t, fm.ReviewRoles, "security")
	assert.NotContains(t, fm.AuditRoles, "implementer")
}

// TestParseFrontmatter_RoleOverrideDisallowedKey covers the warn-and-ignore of a
// key that is neither focus nor enabled inside an override (§2.1): the disallowed
// key is dropped, the focus questions still parse, a lint warning names the
// offending key and the permitted set, and the permitted enabled key never warns.
func TestParseFrontmatter_RoleOverrideDisallowedKey(t *testing.T) {
	spec := `---
tp:
  review_roles:
    tester:
      focus:
        - "Is the empty-panel fallback exercised?"
      enabled: false
      instructions: "you cannot redefine me here"
      severity: "high"
---
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)

	// The permitted focus key still parses; the disallowed keys are ignored.
	assert.Equal(t, []string{"Is the empty-panel fallback exercised?"}, fm.ReviewRoles["tester"].Focus)

	joined := ""
	for _, w := range fm.Warnings {
		joined += w.Message + "\n"
	}
	assert.Contains(t, joined, "tp.review_roles.tester.instructions is not a permitted override key (only focus and enabled); ignored")
	assert.Contains(t, joined, "tp.review_roles.tester.severity is not a permitted override key (only focus and enabled); ignored")
	assert.NotContains(t, joined, "tp.review_roles.tester.enabled", "enabled is a permitted key and never warns as unpermitted")
}

// TestParseFrontmatter_RoleOverrideEnabledBool covers §2.1's boolean enabled: a
// true or false value populates RoleOverride.Enabled, an override without the key
// leaves it unset (nil), and none of the three warns. enabled: true is a no-op —
// the role stays in the map with its focus intact for layering (test 3).
func TestParseFrontmatter_RoleOverrideEnabledBool(t *testing.T) {
	spec := `---
tp:
  review_roles:
    implementer:
      enabled: true
      focus:
        - "Is the retry budget bounded?"
    architect:
      focus:
        - "Does the drop happen outside the corpus resolver?"
  audit_roles:
    data-migration: { enabled: false }
---
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)
	assert.Empty(t, fm.Errors)
	assert.Empty(t, fm.Warnings, "a boolean enabled is permitted and well-formed, so it never warns")

	// enabled: true is parsed, and is a no-op: the role keeps its focus for layering.
	trueOverride := fm.ReviewRoles["implementer"]
	require.NotNil(t, trueOverride.Enabled, "a boolean enabled populates Enabled")
	assert.True(t, *trueOverride.Enabled)
	assert.Equal(t, []string{"Is the retry budget bounded?"}, trueOverride.Focus,
		"enabled: true leaves the role active with its focus layered")

	// enabled: false is parsed too; acting on it is not this parser's job.
	falseOverride := fm.AuditRoles["data-migration"]
	require.NotNil(t, falseOverride.Enabled)
	assert.False(t, *falseOverride.Enabled)
	assert.Equal(t, []string{}, falseOverride.Focus)

	// An override carrying no enabled key stays unset.
	assert.Nil(t, fm.ReviewRoles["architect"].Enabled, "an absent enabled key is unset")
}

// TestParseFrontmatter_RoleOverrideShapeWarnings covers the malformed-value paths:
// a non-mapping override, a non-list focus, and a non-string focus element all
// warn and degrade rather than erroring.
func TestParseFrontmatter_RoleOverrideShapeWarnings(t *testing.T) {
	spec := `---
tp:
  review_roles:
    implementer: "not a mapping"
    tester:
      focus: "not a list"
    architect:
      focus:
        - "valid question"
        - 99
  audit_roles: "not a mapping either"
---
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)

	assert.NotContains(t, fm.ReviewRoles, "implementer", "non-mapping override ignored")
	assert.NotContains(t, fm.ReviewRoles, "tester", "non-list focus ignored")
	assert.Equal(t, []string{"valid question"}, fm.ReviewRoles["architect"].Focus, "non-string element ignored")
	assert.Empty(t, fm.AuditRoles, "a non-mapping audit_roles yields no overrides")

	joined := ""
	for _, w := range fm.Warnings {
		joined += w.Message + "\n"
	}
	assert.Contains(t, joined, "tp.review_roles.implementer is not a mapping")
	assert.Contains(t, joined, "tp.review_roles.tester.focus is not a list")
	assert.Contains(t, joined, "tp.review_roles.architect.focus[1] is not a string")
	assert.Contains(t, joined, "tp.audit_roles is not a mapping")
}

// TestParseFrontmatter_NoRoleOverrides confirms the fields default to empty
// (non-nil) maps when the spec carries no overrides.
func TestParseFrontmatter_NoRoleOverrides(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: prose\n---\n"))
	assert.NotNil(t, fm.ReviewRoles)
	assert.NotNil(t, fm.AuditRoles)
	assert.Empty(t, fm.ReviewRoles)
	assert.Empty(t, fm.AuditRoles)
}

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
	}, fm.ReviewRoles["implementer"])
	assert.Equal(t, []string{}, fm.ReviewRoles["architect"], "an empty focus list is retained as empty, not dropped")
	assert.Equal(t, []string{"Any command injection in the detector cmd?"}, fm.AuditRoles["security"])

	// Overrides never bleed across phases.
	assert.NotContains(t, fm.ReviewRoles, "security")
	assert.NotContains(t, fm.AuditRoles, "implementer")
}

// TestParseFrontmatter_RoleOverrideDisallowedKey covers the warn-and-ignore of a
// key other than focus inside an override (§10.2): the disallowed key is dropped,
// the focus questions still parse, and a lint warning names the offending key.
func TestParseFrontmatter_RoleOverrideDisallowedKey(t *testing.T) {
	spec := `---
tp:
  review_roles:
    tester:
      focus:
        - "Is the empty-panel fallback exercised?"
      instructions: "you cannot redefine me here"
      severity: "high"
---
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)

	// The permitted focus key still parses; the disallowed keys are ignored.
	assert.Equal(t, []string{"Is the empty-panel fallback exercised?"}, fm.ReviewRoles["tester"])

	joined := ""
	for _, w := range fm.Warnings {
		joined += w.Message + "\n"
	}
	assert.Contains(t, joined, "tp.review_roles.tester.instructions is not a permitted override key (only focus); ignored")
	assert.Contains(t, joined, "tp.review_roles.tester.severity is not a permitted override key (only focus); ignored")
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
	assert.Equal(t, []string{"valid question"}, fm.ReviewRoles["architect"], "non-string element ignored")
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

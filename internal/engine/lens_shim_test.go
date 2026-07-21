package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func joinWarnings(ws []string) string {
	return strings.Join(ws, "\n")
}

// TestTranslateLegacyLens_All covers lens.all fanning out to every active review
// role's focus with a deprecation warning (§10.4).
func TestTranslateLegacyLens_All(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: prose\n  lens:\n    all:\n      - \"Does any chapter leak a plot point?\"\n---\n"))
	active := []string{"coherence", "soundness"}

	overrides, warnings := TranslateLegacyLens(fm, active)
	assert.Equal(t, []string{"Does any chapter leak a plot point?"}, overrides["coherence"])
	assert.Equal(t, []string{"Does any chapter leak a plot point?"}, overrides["soundness"])
	assert.Contains(t, joinWarnings(warnings), "tp: lens is deprecated", "a translated lens emits a deprecation warning")
}

// TestTranslateLegacyLens_Role covers a lens.<role-id> appending to only that
// active review role, and the lens.all-then-role-specific ordering.
func TestTranslateLegacyLens_Role(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  lens:\n    all:\n      - \"shared question\"\n    implementer:\n      - \"role question\"\n---\n"))
	active := []string{"implementer", "tester", "architect"}

	overrides, warnings := TranslateLegacyLens(fm, active)
	assert.Equal(t, []string{"shared question", "role question"}, overrides["implementer"], "all first, then role-specific")
	assert.Equal(t, []string{"shared question"}, overrides["tester"], "tester gets only lens.all")
	assert.Equal(t, []string{"shared question"}, overrides["architect"])
	assert.Contains(t, joinWarnings(warnings), "deprecated")
}

// TestTranslateLegacyLens_UnknownID warns when a lens.<role-id> is not an active
// review role and translates nothing for it (§10.4).
func TestTranslateLegacyLens_UnknownID(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  lens:\n    architect:\n      - \"unmatched question\"\n---\n"))
	active := []string{"implementer", "tester"} // no architect

	overrides, warnings := TranslateLegacyLens(fm, active)
	assert.NotContains(t, overrides, "architect")
	assert.Empty(t, overrides["implementer"])
	joined := joinWarnings(warnings)
	assert.Contains(t, joined, "deprecated")
	assert.Contains(t, joined, `lens.architect targets "architect" which is not an active review role`)
}

// TestTranslateLegacyLens_NewFormWins covers the rule that when the new
// review_roles/audit_roles form is present, the legacy lens is ignored with a
// warning and never merged (§10.4).
func TestTranslateLegacyLens_NewFormWins(t *testing.T) {
	spec := "---\ntp:\n  review_roles:\n    implementer:\n      focus:\n        - \"new-form question\"\n  lens:\n    all:\n      - \"legacy question\"\n---\n"
	fm := ParseFrontmatterBytes([]byte(spec))
	require.NotEmpty(t, fm.ReviewRoles, "new form parsed")

	overrides, warnings := TranslateLegacyLens(fm, []string{"implementer"})
	assert.Empty(t, overrides, "no lens-derived overrides when the new form is present")
	joined := joinWarnings(warnings)
	assert.Contains(t, joined, "legacy tp: lens is ignored")
	assert.NotContains(t, joined, "auto-translating", "no three-way merge / no deprecation-translate path")
}

// TestTranslateLegacyLens_NoLens returns empty with no warnings when the spec
// carries no lens block.
func TestTranslateLegacyLens_NoLens(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: software\n---\n"))
	overrides, warnings := TranslateLegacyLens(fm, []string{"implementer"})
	assert.Empty(t, overrides)
	assert.Empty(t, warnings)
}

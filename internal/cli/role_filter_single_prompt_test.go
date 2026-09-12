package cli_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// singlePromptModes are the five tp review modes that emit exactly one prompt:
// the four --perspective values and --verify. The keys are legalModeCases's,
// so the mode's own role and its arguments are taken from §4.2.2's own table
// rather than restated here.
//
// The value is a name tp recognises that the mode never emits. It is
// `implementer` in every row on purpose: `tp review <spec> --perspective
// regression --role implementer` is the reproduction in BUGS.md, and the four
// siblings are the same command shape.
var singlePromptModes = map[string]string{
	"review/perspective/regression":    "implementer",
	"review/perspective/code-audit":    "implementer",
	"review/perspective/documentation": "implementer",
	"review/perspective/testing":       "implementer",
	"review/verify":                    "implementer",
}

// singlePromptCase resolves one row of singlePromptModes against §4.2.2's
// table, and fails if the mode does not in fact emit exactly one prompt — the
// premise every assertion below rests on.
func singlePromptCase(t *testing.T, dir string, cases map[string]legalModeCase, name string) legalModeCase {
	t.Helper()
	c, found := cases[name]
	require.True(t, found, "%s must be a mode §4.2.2 calls legal", name)

	full := readNarrowedEmission(t, dir, c.args...)
	require.Equal(t, []string{c.ownRole}, full.roles, "%s emits exactly one prompt, and it is %s", name, c.ownRole)
	return c
}

// withRole is c.args plus a --role selection and whatever else the case needs,
// built on a fresh slice so the table's own args are never appended into.
func withRole(args []string, role string, extra ...string) []string {
	out := slices.Clone(args)
	out = append(out, "--role", role)
	return append(out, extra...)
}

// TestSinglePromptModeNamesTheRoleRoleFilterNarrowedAway is the defect in
// BUGS.md: `tp review <spec> --perspective regression --role implementer` drops
// the regression prompt with nothing in the payload saying so.
//
// The rule is the panel's: a role the round emitted and --role narrowed away is
// named in skipped_roles with reason role-filter. Here the emission is one
// prompt, so a foreign --role narrows that single prompt away and the mode's own
// role is what the entry names.
func TestSinglePromptModeNamesTheRoleRoleFilterNarrowedAway(t *testing.T) {
	t.Parallel()
	dir, cases := legalModeCases(t)

	for name, foreign := range singlePromptModes {
		t.Run(name, func(t *testing.T) {
			c := singlePromptCase(t, dir, cases, name)
			require.NotEqual(t, c.ownRole, foreign, "the foreign role must differ from the mode's own")

			narrowed := readNarrowedEmission(t, dir, withRole(c.args, foreign)...)
			assert.Empty(t, narrowed.roles, "%s emits nothing under a role it does not carry", name)
			require.True(t, narrowed.present, "%s: skipped_roles is in the payload", name)
			assert.Equal(t, map[string]string{c.ownRole: roleFilterReason}, narrowed.skipped,
				"%s: the prompt --role narrowed away is named, as the panel names its own", name)
		})
	}
}

// TestSinglePromptModeSkipsNothingWhenRoleNamesItsOwnPrompt is the second half
// of the rule: when --role names the one prompt the mode emits, nothing was
// narrowed away, so the array is empty rather than naming the role the caller
// was handed.
func TestSinglePromptModeSkipsNothingWhenRoleNamesItsOwnPrompt(t *testing.T) {
	t.Parallel()
	dir, cases := legalModeCases(t)

	for name := range singlePromptModes {
		t.Run(name, func(t *testing.T) {
			c := singlePromptCase(t, dir, cases, name)

			kept := readNarrowedEmission(t, dir, withRole(c.args, c.ownRole)...)
			require.Equal(t, []string{c.ownRole}, kept.roles, "%s keeps the prompt that was asked for", name)
			require.True(t, kept.present, "%s: skipped_roles is in the payload", name)
			assert.Empty(t, kept.skipped, "%s: the emitted prompt is the one kept, so nothing is skipped", name)
		})
	}
}

// TestSinglePromptModeWithoutRoleCarriesAnEmptySkippedRoles is the control.
//
// The key is PRESENT and empty rather than absent, which is the panel's shape:
// tp review's default mode emits `skipped_roles` on every payload it writes
// without --compact, and a reader that has to branch on the key's existence per
// mode is exactly the drift this defect is. --compact with no --role still omits
// it (§8.4), unchanged.
func TestSinglePromptModeWithoutRoleCarriesAnEmptySkippedRoles(t *testing.T) {
	t.Parallel()
	dir, cases := legalModeCases(t)

	for name := range singlePromptModes {
		t.Run(name, func(t *testing.T) {
			c := singlePromptCase(t, dir, cases, name)

			full := readNarrowedEmission(t, dir, c.args...)
			assert.True(t, full.present, "%s: skipped_roles is present without --compact", name)
			assert.Empty(t, full.skipped, "%s: nothing narrowed, nothing skipped", name)

			compact := readNarrowedEmission(t, dir, append(slices.Clone(c.args), "--compact")...)
			require.Equal(t, []string{c.ownRole}, compact.roles, "%s: --compact still emits the prompt", name)
			assert.False(t, compact.present, "%s: --compact without --role omits skipped_roles", name)
		})
	}
}

// TestSinglePromptModeSkipsSurviveCompact applies skippedRolesSurviveCompact's
// rule to these modes: a non-empty skipped_roles survives --compact under
// --role, because it is the only thing telling a compact caller the payload is
// a slice of the round; an empty one under --role is still omitted.
func TestSinglePromptModeSkipsSurviveCompact(t *testing.T) {
	t.Parallel()
	dir, cases := legalModeCases(t)

	for name, foreign := range singlePromptModes {
		t.Run(name, func(t *testing.T) {
			c := singlePromptCase(t, dir, cases, name)

			narrowed := readNarrowedEmission(t, dir, withRole(c.args, foreign, "--compact")...)
			assert.Empty(t, narrowed.roles, "%s: the foreign role emits nothing", name)
			require.True(t, narrowed.present, "%s: --compact keeps a non-empty skipped_roles under --role", name)
			assert.Equal(t, map[string]string{c.ownRole: roleFilterReason}, narrowed.skipped, "%s", name)

			kept := readNarrowedEmission(t, dir, withRole(c.args, c.ownRole, "--compact")...)
			require.Equal(t, []string{c.ownRole}, kept.roles, "%s", name)
			assert.False(t, kept.present, "%s: an empty skipped_roles is omitted under --compact, as on the panel", name)
		})
	}
}

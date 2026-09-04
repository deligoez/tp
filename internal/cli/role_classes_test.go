package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// skipCase is one of §4.2.1's five reasons, built on the command that can
// produce it.
//
// The reasons are a UNION across both commands, not a matrix either one can
// produce: `no-baseline` is tp review's built-in regression role and
// `no-spec-change` needs --diff-from, which tp audit does not have. So each
// case names its own command, and a table that ran all five against both would
// be asserting cases the code cannot reach.
type skipCase struct {
	reason string
	setUp  func(t *testing.T) (dir string, args []string, role string)
}

// TestEverySkipReasonExitsZeroUnderRole is the exit-0 half of §6.2 property 5,
// exercised once per reason.
//
// Each case is verified twice over: the unrestricted emission must actually
// carry that role with that reason -- otherwise the fixture built something
// else and the --role assertion would pass for the wrong cause -- and then
// `--role <that role>` must exit 0 with an empty prompts[] echoing the reason.
func TestEverySkipReasonExitsZeroUnderRole(t *testing.T) {
	t.Parallel()
	for _, c := range skipCases() {
		t.Run(c.reason, func(t *testing.T) {
			dir, args, role := c.setUp(t)

			// 1. The fixture really produces this reason.
			stdout, stderr, code := runTP(t, dir, args...)
			require.Equal(t, 0, code, "the fixture must emit: %s", stderr)

			byReason := map[string]string{}
			for _, s := range skippedRolesFrom(t, stdout) {
				byReason[s["role"].(string)] = s["reason"].(string)
			}
			require.Equal(t, c.reason, byReason[role],
				"the fixture must skip %q with reason %q, not something else", role, c.reason)

			// 2. --role on it exits 0, empty, with the reason still named.
			narrowed, narrowErr, narrowCode := runTP(t, dir, append(args, "--role", role)...)
			require.Equal(t, 0, narrowCode,
				"a role skipped %s is recognised, so --role exits 0: %s", c.reason, narrowErr)

			var payload struct {
				Prompts []json.RawMessage `json:"prompts"`
			}
			require.NoError(t, json.Unmarshal([]byte(narrowed), &payload))
			assert.Empty(t, payload.Prompts, "the skipped role emits no prompt")
			assert.NotNil(t, payload.Prompts, "prompts stays an array, never null")

			got := map[string]string{}
			for _, s := range skippedRolesFrom(t, narrowed) {
				got[s["role"].(string)] = s["reason"].(string)
			}
			assert.Equal(t, c.reason, got[role],
				"the --role payload echoes %q's own reason", role)
		})
	}
}

// TestUnrecognisedNameIsDistinguishableFromASkippedOne is the exit-2 half, and
// the reason the exit-0 half above is not enough on its own: an implementation
// that exited 0 for every name would satisfy all five cases and tell a caller
// nothing about a typo.
func TestUnrecognisedNameIsDistinguishableFromASkippedOne(t *testing.T) {
	t.Parallel()
	dir, args, skipped := skipDisabledBySpec(t)

	_, _, okCode := runTP(t, dir, append(args, "--role", skipped)...)
	require.Equal(t, 0, okCode, "the skipped role is recognised")

	_, stderr, code := runTP(t, dir, append(args, "--role", skipped+"-typo")...)
	require.Equal(t, 2, code, "a name tp recognises nowhere is a usage error")

	var refusal struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(stderr), &refusal), "the refusal is JSON: %s", stderr)
	assert.Contains(t, refusal.Error, skipped+"-typo", "the error repeats what was typed")
	assert.NotEmpty(t, refusal.Hint, "the refusal carries a hint naming what would have been emitted")
	assert.Contains(t, refusal.Hint, skipped,
		"the hint names the skipped role too, which is what makes a typo distinguishable from it")
}

func skipCases() []skipCase {
	return []skipCase{
		{engine.SkipNoBaseline, skipNoBaseline},
		{engine.SkipDisabledBySpec, skipDisabledBySpec},
		{engine.SkipDomainMismatch, skipDomainMismatch},
		{engine.SkipNoChecklistItems, skipNoChecklistItems},
		{engine.SkipNoSpecChange, skipNoSpecChange},
	}
}

// no-baseline: tp review only. The built-in regression role has no
// snapshot-round-0.md to diff against, so round 1 skips it. It is never a unit.
func skipNoBaseline(t *testing.T) (dir string, args []string, role string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	return dir, []string{"review", "spec.md"}, engine.RegressionRoleID
}

// disabled-by-spec: either command; tp review here.
func skipDisabledBySpec(t *testing.T) (dir string, args []string, role string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750))
	writeReviewerRole(t, dir, "keeper.json",
		`{"id":"keeper","title":"Keeper","instructions":"You review.","focus":["q"]}`)
	writeReviewerRole(t, dir, "dropped.json",
		`{"id":"dropped","title":"Dropped","instructions":"You review.","focus":["q"]}`)
	spec := "---\ntp:\n  review_roles:\n    dropped:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	return dir, []string{"review", "spec.md"}, "dropped"
}

// domain-mismatch: a corpus role whose domains omit the spec's domain.
func skipDomainMismatch(t *testing.T) (dir string, args []string, role string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750))
	writeReviewerRole(t, dir, "universal.json",
		`{"id":"universal","title":"Universal","instructions":"You review.","focus":["q"]}`)
	writeReviewerRole(t, dir, "prose-only.json",
		`{"id":"prose-only","title":"Prose","instructions":"You review.","focus":["q"],"domains":["prose"]}`)
	spec := "---\ntp:\n  domain: software\n---\n# Spec\n## 1. A\ncontent\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	return dir, []string{"review", "spec.md"}, "prose-only"
}

// no-checklist-items: tp audit. Every affected file is dropped by the universe
// filter, so the shared code-file list is empty and the non-spec-coverage roles
// earn no items -- while still being spawned as units, which is the row §4.2.1
// puts in bold.
func skipNoChecklistItems(t *testing.T) (dir string, args []string, role string) {
	t.Helper()
	dir = writeAuditorCorpusProject(t, routingSpec, "spec-coverage", "shared-arm")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "testdata"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "testdata", "fixture.go"),
		[]byte("package main\n"), 0o600))
	return dir, []string{"audit", "spec.md", "--affected-files", filepath.Join("testdata", "fixture.go")}, "shared-arm"
}

// no-spec-change: tp review under --diff-from only, and the only reason tp
// audit cannot produce -- it registers no --diff-from at all.
func skipNoSpecChange(t *testing.T) (dir string, args []string, role string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750))

	// The focus must carry an explicit §N reference: engine.RoleFocusOutsideDiff
	// scopes a role to a section ONLY through tp's canonical §N.M syntax, and a
	// generic focus keeps the role emitted because the reviewer self-scopes
	// against the changed-sections block. A first version of this fixture used
	// a bare word and produced no skip at all.
	writeReviewerRole(t, dir, "widgets.json",
		`{"id":"widgets","title":"Widgets","instructions":"You review.","focus":["§1 does the widget rule hold?"]}`)
	writeReviewerRole(t, dir, "gadgets.json",
		`{"id":"gadgets","title":"Gadgets","instructions":"You review.","focus":["§2 does the gadget rule hold?"]}`)

	// The baseline and the spec differ in §1 only, so the gadgets role's focus
	// falls entirely outside every changed section.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "baseline.md"),
		[]byte("# Spec\n## 1. Widgets\nold widgets\n## 2. Gadgets\ngadgets stay\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. Widgets\nnew widgets text\n## 2. Gadgets\ngadgets stay\n"), 0o600))
	return dir, []string{"review", "spec.md", "--diff-from", "baseline.md"}, "gadgets"
}

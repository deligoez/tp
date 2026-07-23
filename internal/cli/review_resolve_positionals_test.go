package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewResolve_RejectsSpecPositionalExit2 covers §4.1: --resolve takes the
// findings NDJSON as the positional, so a spec-looking positional where the
// findings file is expected exits 2 naming the expected form. Today
// `tp review <spec> --resolve f.ndjson 0 fixed "why"` reads the index as the
// disposition — this guards against that regression.
func TestReviewResolve_RejectsSpecPositionalExit2(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\nbody\n"), 0o600))
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"severity":"low","finding":"x"}`+"\n"), 0o600))

	// Old broken form: spec as positional, findings among the trailing args.
	_, stderr, code := runTP(t, dir, "review", spec, "--resolve", findings, "0", "fixed", "why")
	require.Equal(t, 2, code, "spec positional must be rejected: %s", stderr)
	assert.Contains(t, stderr, "looks like a spec")
	assert.Contains(t, stderr, "findings NDJSON as the positional")
	// Must not misread the index as the disposition.
	assert.NotContains(t, stderr, "invalid status")
}

// TestReviewResolve_NonNumericIndexNamesExpectedForm covers §4.3: a non-numeric
// index is a usage error (exit 2) that names the expected form, not an
// "invalid status" error. The 0-based base is stated in the message.
func TestReviewResolve_NonNumericIndexNamesExpectedForm(t *testing.T) {
	dir := t.TempDir()
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"severity":"low","finding":"x"}`+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", findings, "--resolve", "abc", "fixed", "why")
	require.Equal(t, 2, code, "non-numeric index must exit 2: %s", stderr)
	assert.Contains(t, stderr, "invalid index")
	assert.Contains(t, stderr, "0-based")
	assert.Contains(t, stderr, "findings.ndjson")
	assert.NotContains(t, stderr, "invalid status", "index error must not be misreported as a status error")
}

// TestReviewResolve_HappyPathUnchanged confirms the findings-positional shape
// still resolves a finding end-to-end after the §4.1/§4.3 changes.
func TestReviewResolve_HappyPathUnchanged(t *testing.T) {
	dir := t.TempDir()
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"severity":"low","finding":"x"}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", findings, "--resolve", "0", "fixed", "done")
	require.Equal(t, 0, code, "happy path must still work: %s", stderr)
	assert.Contains(t, stdout, `"status": "fixed"`)
}

// TestReviewResolveAll_RejectsSpecPositionalExit2 covers §4.1 for --resolve-all:
// a spec-looking positional where the findings file is expected exits 2 naming
// the expected form.
func TestReviewResolveAll_RejectsSpecPositionalExit2(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\nbody\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", spec, "--resolve-all", "fixed", "why")
	require.Equal(t, 2, code, "spec positional must be rejected: %s", stderr)
	assert.Contains(t, stderr, "looks like a spec")
	assert.Contains(t, stderr, "findings NDJSON as the positional")
}

// TestReviewResolve_ResolveUsageStatesZeroBased covers §4.3: the --help text
// for --resolve states that indices are 0-based.
func TestReviewResolve_ResolveUsageStatesZeroBased(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := runTP(t, dir, "review", "--help")
	require.True(t, code == 0 || code == 2, "help should not hard-fail: code=%d", code)
	assert.Contains(t, stdout, "0-based")
}

// TestSpecScopedModesStillRequireSpec covers §4.2: the spec positional stays
// required for --record, --status, and --verify (all spec-scoped). The §4.1
// changes must not relax these.
func TestSpecScopedModesStillRequireSpec(t *testing.T) {
	dir := t.TempDir()
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"severity":"low","finding":"x"}`+"\n"), 0o600))

	for _, args := range [][]string{
		{"review", "--verify", "--findings", findings},
		{"review", "--record", findings},
		{"review", "--status"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 2, code, "spec-scoped mode %v must require the spec positional: %s", args, stderr)
		assert.Contains(t, stderr, "spec path required")
	}
}

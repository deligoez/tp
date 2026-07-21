package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBadRole writes an invalid role file (id disagrees with the filename stem).
func writeBadRole(t *testing.T, dir, phase string) {
	t.Helper()
	phaseDir := filepath.Join(dir, ".tp", phase)
	require.NoError(t, os.MkdirAll(phaseDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(phaseDir, "bad.json"), []byte(`{"id":"other","title":"T","instructions":"I"}`), 0o600))
}

// TestLint_AbortsOnBadReviewer: tp lint validates the reviewer corpus and aborts
// with exit 3 and a repair-or-delete hint (§3.6).
func TestLint_AbortsOnBadReviewer(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	writeBadRole(t, dir, "reviewers")

	_, stderr, code := runTP(t, dir, "lint", "spec.md")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "repair or delete")
	assert.Contains(t, stderr, "bad.json")
}

// TestLint_AbortsOnBadAuditor: tp lint aborts on a bad auditor too (§3.6).
func TestLint_AbortsOnBadAuditor(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	writeBadRole(t, dir, "auditors")

	_, stderr, code := runTP(t, dir, "lint", "spec.md")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "repair or delete")
}

// TestReview_AbortsOnBadReviewer: tp review aborts on a malformed reviewer with
// exit 3 (§3.6).
func TestReview_AbortsOnBadReviewer(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	writeBadRole(t, dir, "reviewers")

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "repair or delete")
}

// TestAudit_BadAuditorDoesNotBlockReview: a malformed auditor aborts tp audit but
// never blocks tp review — phase independence (§3.6).
func TestAudit_BadAuditorDoesNotBlockReview(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. X\n| A | B |\n|---|---|\n| 1 | 2 |\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
	writeBadRole(t, dir, "auditors")

	// A bad auditor does not block review (no bad reviewer).
	_, _, rcode := runTP(t, dir, "review", "spec.md", "--no-state")
	assert.Equal(t, 0, rcode, "a bad auditor must not block review")

	// But it aborts audit with exit 3.
	_, astderr, acode := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	assert.Equal(t, 3, acode)
	assert.Contains(t, astderr, "repair or delete")
}

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmission_ReviewOutputContract: every review prompt instructs the sub-agent
// to stamp role, location (a §-anchor), class, and severity (§7.3).
func TestEmission_ReviewOutputContract(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. X\ncontent\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := reviewPromptsByRole(t, stdout)
	impl := byRole["implementer"]

	assert.Contains(t, impl, "Output contract")
	assert.Contains(t, impl, `role: "implementer"`, "the sub-agent stamps its own role for attribution")
	assert.Contains(t, impl, "§3.2", "location is a section anchor")
	assert.Contains(t, impl, "class:")
	assert.Contains(t, impl, "severity:")
	assert.NotContains(t, impl, "status: one of PASS", "review findings carry no status")
}

// TestEmission_AuditOutputContract: every audit prompt additionally instructs the
// sub-agent to stamp status ∈ PASS/PARTIAL/FAIL (§7.3).
func TestEmission_AuditOutputContract(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. Widgets\n| Name | Type |\n|------|------|\n| a | b |\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := auditPromptsByRole(t, stdout)
	spec := byRole["spec-coverage"]["prompt"].(string)

	assert.Contains(t, spec, "Output contract")
	assert.Contains(t, spec, `role: "spec-coverage"`)
	assert.Contains(t, spec, "§3.2", "audit findings also carry a section-anchor location")
	assert.Contains(t, spec, "class:")
	assert.Contains(t, spec, "severity:")
	assert.Contains(t, spec, "status: one of PASS, PARTIAL, FAIL")
}

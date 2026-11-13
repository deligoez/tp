package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedAuditArgs writes n .go files into dir and returns the audit arguments
// naming every one of them with --affected-files.
func namedAuditArgs(t *testing.T, dir string, n int) []string {
	t.Helper()
	args := []string{"audit", "spec.md"}
	for i := range n {
		name := fmt.Sprintf("n%02d.go", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
		args = append(args, "--affected-files", name)
	}
	return args
}

// TestAuditNamedList_GoesWholeToTheCodeRoles: decision 2. Twenty-five named
// files used to give every code role "10 of 25" while file_summary said
// truncated false; now each code role gets all twenty-five, its header says
// the list is named, and the payload sizes it.
func TestAuditNamedList_GoesWholeToTheCodeRoles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))

	stdout, stderr, code := runTP(t, dir, namedAuditArgs(t, dir, 25)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]
	assert.Equal(t, float64(25), sec["checklist_count"], "one file_check item per named file")
	assert.Len(t, sec["affected_files"].([]any), 25)
	assert.Equal(t, float64(25), sec["file_count"], "the payload sizes the list")
	assert.Equal(t, float64(25), sec["file_pool"], "and names the pool it came from")
	assert.Contains(t, sec["prompt"].(string), "## Affected Files (all 25 named)\n")
	assert.NotContains(t, sec["prompt"].(string), "(10 of 25)")
}

// TestAuditNamedList_SizesSurviveCompact: --compact drops affected_files, so
// the two counts are what is left to show the cost before fan-out.
func TestAuditNamedList_SizesSurviveCompact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))

	stdout, stderr, code := runTP(t, dir, append(namedAuditArgs(t, dir, 12), "--compact")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]
	assert.NotContains(t, sec, "affected_files", "--compact still drops the duplicate list")
	assert.Equal(t, float64(12), sec["file_count"])
	assert.Equal(t, float64(12), sec["file_pool"])
}

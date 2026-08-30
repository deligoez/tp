package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Section 8a.3: tp audit's auto-detect caps the audited file set at 50. The cap
// used to be reported on stderr alone, so file_summary.total_files read as 50
// with nothing in the payload to separate a truncated set from a complete one,
// and --quiet erased the only signal there was. These tests pin the
// payload-side accounting: truncated, total_changed, and a notice that names
// both numbers.

// auditRepoWithChangedFiles builds an audit repo whose staged diff holds n
// auditable .go files, so auto-detection sees exactly n changed files.
func auditRepoWithChangedFiles(t *testing.T, n int) (dir, specPath string) {
	t.Helper()
	dir = t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("f%03d.go", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
	}
	git(t, dir, "add", "-A")
	return dir, specPath
}

// auditFileSummary decodes an audit payload and returns its file_summary.
func auditFileSummary(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	fs, ok := result["file_summary"].(map[string]any)
	require.True(t, ok, "file_summary is present in the audit payload")
	return fs
}

// A capped set says so in the payload, sizes the loss, and keeps total_files
// reporting what was actually audited.
func TestAuditTruncated_PayloadCarriesTheCap(t *testing.T) {
	dir, specPath := auditRepoWithChangedFiles(t, 63)

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code)

	fs := auditFileSummary(t, stdout)
	assert.Equal(t, true, fs["truncated"], "a capped file set reports truncated")
	assert.Equal(t, float64(63), fs["total_changed"], "total_changed is the pre-cap count")
	assert.Equal(t, float64(50), fs["total_files"], "total_files keeps reporting the audited count")

	files, _ := json.Marshal(fs)
	assert.Contains(t, stderr, "63", "the notice names the changed count: %s", files)
	assert.Contains(t, stderr, "50", "the notice names the audited count")
}

// --quiet erases the notice; it cannot erase the flag, because the flag is in
// the payload.
func TestAuditTruncated_SurvivesQuiet(t *testing.T) {
	dir, specPath := auditRepoWithChangedFiles(t, 63)

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--quiet")
	require.Equal(t, 0, code)
	assert.NotContains(t, stderr, "auditing first", "--quiet erases the stderr notice")

	fs := auditFileSummary(t, stdout)
	assert.Equal(t, true, fs["truncated"], "--quiet cannot erase the truncation flag")
	assert.Equal(t, float64(63), fs["total_changed"])
	assert.Equal(t, float64(50), fs["total_files"])
}

// An uncapped set reports truncated false with total_changed equal to
// total_files, and emits no notice.
func TestAuditNotTruncated_ReportsTheWholeSet(t *testing.T) {
	dir, specPath := auditRepoWithChangedFiles(t, 3)

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code)

	fs := auditFileSummary(t, stdout)
	assert.Equal(t, false, fs["truncated"], "an uncapped file set is not truncated")
	assert.Equal(t, float64(3), fs["total_changed"])
	assert.Equal(t, float64(3), fs["total_files"])
	assert.NotContains(t, stderr, "auditing first", "no cap, no notice")
}


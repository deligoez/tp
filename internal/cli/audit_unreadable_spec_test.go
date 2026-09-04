package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditUnreadableSpecHint: runAudit's os.Stat guard only rejects a MISSING
// spec, so a spec that exists but cannot be read falls through to
// loadAuditSpec's os.ReadFile. That site passed no hint at all, so it inherited
// the code-3 default — TASK-file advice ("run 'tp use <file>' … 'tp init
// <spec>'"), the wrong object entirely for a permission failure on the spec.
// specFileMissingHint would be wrong here too: the caller did not mistype the
// path. The hint carries the real cause.
func TestAuditUnreadableSpecHint(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission bits this test relies on")
	}
	dir := t.TempDir()

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))
	require.NoError(t, os.Chmod(specPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(specPath, 0o600) })

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 3, code, "an unreadable spec is a file error")

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Contains(t, payload["error"], specPath, "the error names the spec tp could not read")

	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "permission denied", "the hint carries the real cause")
	assert.NotContains(t, hint, "tp use", "task-file advice is the wrong object for an unreadable spec")
	assert.NotContains(t, hint, "tp init", "task-file advice is the wrong object for an unreadable spec")
}

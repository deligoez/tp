package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specFileMissingHint's doc comment draws the boundary these two tests pin
// down: the const belongs to tp's FIRST contact with a spec path, and a site
// reached AFTER that path already read successfully must pass err.Error() so
// the real I/O cause reaches the agent instead of the code-3 task-file default
// ("run 'tp use <file>' … 'tp init <spec>'") a hintless site inherits.

// TestReviewUnreadableSpecKeepsPathHint: `tp review` has no os.Stat guard on
// the spec (tp audit does, at runAudit), so resolveReviewSpecContent's read IS
// tp's first contact with the path. A permission failure and a typo are
// indistinguishable there, so the spec-path hint is correct — what must never
// appear is the task-file default.
func TestReviewUnreadableSpecKeepsPathHint(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the read never fails")
	}
	dir := t.TempDir()

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## One\n\ntext\n"), 0o600))
	require.NoError(t, os.Chmod(specPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(specPath, 0o600) })

	_, stderr, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 3, code, "an unreadable spec is a file error")

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Contains(t, payload["error"], specPath, "the error names the spec tp could not read")

	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "spec path", "a first-contact failure points at the path itself")
	assert.NotContains(t, hint, "tp use", "task-file advice is the wrong object for an unreadable spec")
	assert.NotContains(t, hint, "tp init", "task-file advice is the wrong object for an unreadable spec")
}


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
	t.Parallel()
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

// TestReviewSnapshotWriteFailureHint: loadReviewRoundState's round snapshot
// runs long after resolveReviewSpecContent read the same spec, so a failure
// there is a state-directory I/O problem, never a mistyped path. The site
// passed no hint at all and so answered a read-only .tp-review/ with task-file
// advice for a spec path that was perfectly correct — the exact defect
// 85e8824 fixed on the audit side.
func TestReviewSnapshotWriteFailureHint(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory permission bits this test relies on")
	}
	dir := t.TempDir()

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## One\n\ntext\n"), 0o600))

	// Round 1 creates .tp-review/<slug>/ and its state index.
	_, stderr, code := runTP(t, dir, "review", specPath)
	require.Equal(t, 0, code, "round 1 emission must succeed: %s", stderr)

	entries, err := os.ReadDir(filepath.Join(dir, ".tp-review"))
	require.NoError(t, err)
	require.Len(t, entries, 1, "one state directory per spec")
	stateDir := filepath.Join(dir, ".tp-review", entries[0].Name())

	// Read-only state directory: state.json still loads, the snapshot cannot
	// be written.
	require.NoError(t, os.Chmod(stateDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(stateDir, 0o700) })

	_, stderr, code = runTP(t, dir, "review", specPath)
	require.Equal(t, 3, code, "an unwritable state directory is a file error")

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Contains(t, payload["error"], specPath, "the error names the spec whose round could not be snapshotted")

	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "permission denied", "the hint carries the real cause")
	assert.NotContains(t, hint, "tp use", "task-file advice is the wrong object for a state-write failure")
	assert.NotContains(t, hint, "tp init", "task-file advice is the wrong object for a state-write failure")
	assert.NotContains(t, hint, "spec path", "the caller did not mistype the spec path")
}

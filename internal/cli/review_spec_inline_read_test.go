package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewSpecInlineMissingSpecFailsLoudly: readSpecContent answered every
// os.Open error with "", and resolveReviewSpecContent's specInline branch fed
// that straight into the prompt as a legitimately empty spec — while its
// sibling branches exit ExitFile. So `tp review nope.md --spec-inline` exited 0
// with an empty "Spec content:" block, and the SAME command without
// --spec-inline exited 3. The read error must reach the caller.
func TestReviewSpecInlineMissingSpecFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	gPath := filepath.Join(dir, "g.go")
	require.NoError(t, os.WriteFile(gPath, []byte("package main\n"), 0o600))

	missing := filepath.Join(dir, "nope.md")
	stdout, stderr, code := runTP(t, dir, "review", missing,
		"--perspective", "code-audit", "--spec-inline", "--affected-files", gPath)
	assert.Equal(t, 3, code, "--spec-inline must not turn a missing spec into an empty one")
	assert.Contains(t, stderr, missing, "stderr names the spec tp could not read")
	assert.NotContains(t, stdout, "Spec content:", "no prompt is emitted from a spec tp never read")
}

// TestReviewSpecInlineUnreadableSpecFailsLoudly: the same guard must hold for a
// spec that exists but cannot be opened — the case an up-front existence check
// would miss.
func TestReviewSpecInlineUnreadableSpecFailsLoudly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the open never fails")
	}
	dir := t.TempDir()
	gPath := filepath.Join(dir, "g.go")
	require.NoError(t, os.WriteFile(gPath, []byte("package main\n"), 0o600))

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## 1. Models\nCreate a Task model.\n"), 0o600))
	require.NoError(t, os.Chmod(specPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(specPath, 0o600) })

	_, stderr, code := runTP(t, dir, "review", specPath,
		"--perspective", "code-audit", "--spec-inline", "--affected-files", gPath)
	assert.Equal(t, 3, code, "an unreadable spec is a file error, not an empty spec")
	assert.Contains(t, stderr, specPath, "stderr names the spec tp could not read")
}

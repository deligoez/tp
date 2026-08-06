package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewUnreadableFindingsFileFailsLoudly guards the findings read path:
// parseFindingsFile used to answer every os.Open error with an empty slice,
// and the upstream guard rejects only a missing path. An existing-but-
// unreadable --findings file therefore produced exit 0, empty stderr, and
// previous_findings: 0 — every prior finding, critical ones included, dropped
// out of the emitted prompts with no signal at all. The read error must reach
// the caller and abort with tp's file exit code, naming the path.
func TestReviewUnreadableFindingsFileFailsLoudly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the open never fails")
	}
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath,
		[]byte(`{"category":"correctness","severity":"critical","location":"§1.1","finding":"missing invariant"}`+"\n"), 0o600))
	require.NoError(t, os.Chmod(findingsPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(findingsPath, 0o600) })

	stdout, stderr, exit := runTP(t, dir, "review", specPath, "--no-state", "--round", "2", "--findings", findingsPath)
	assert.Equal(t, 3, exit, "an unreadable findings file is a file error, not a silent empty set")
	assert.Contains(t, stderr, findingsPath, "stderr names the file tp could not read")
	assert.NotContains(t, stdout, "\"prompts\"", "no prompts are emitted from findings tp never read")
}

// TestReviewRegressionUnreadableFindingsFileFailsLoudly: the standalone
// regression path reads the same file through the same helper, so it must fail
// the same way rather than report an empty fixed-findings list.
func TestReviewRegressionUnreadableFindingsFileFailsLoudly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the open never fails")
	}
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n"), 0o600))
	basePath := filepath.Join(dir, "base.md")
	require.NoError(t, os.WriteFile(basePath, []byte("# Spec\n## 1. Models\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath,
		[]byte(`{"category":"correctness","severity":"high","location":"§1.1","finding":"x","resolved":{"status":"fixed"}}`+"\n"), 0o600))
	require.NoError(t, os.Chmod(findingsPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(findingsPath, 0o600) })

	_, stderr, exit := runTP(t, dir, "review", specPath, "--perspective", "regression",
		"--diff-from", basePath, "--findings", findingsPath)
	assert.Equal(t, 3, exit, "the regression path aborts on an unreadable findings file too")
	assert.Contains(t, stderr, findingsPath, "stderr names the file tp could not read")
}

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// The tests below are spec/backlog/role-files-in-state-dir.md: outside a run,
// the file a review or audit role writes lives in the spec's own state
// directory, so its identity comes from location.

// TestSameBaseSpecsInTwoDirectoriesGetTwoRoleFiles is the collision closed:
// x/a.md and y/a.md emitted from one working directory named one
// review-a-r1-<role>.ndjson per role.
func TestSameBaseSpecsInTwoDirectoriesGetTwoRoleFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, sub := range []string{"x", "y"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub, "a.md"), []byte("# Spec\n## 1. Models\nA Task model.\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub, "code.go"), []byte("package main\n"), 0o600))
	}
	// One repository holds both, so both specs are this project's.
	initGitRepo(t, dir)
	for _, phase := range []string{"review", "audit"} {
		t.Run(phase, func(t *testing.T) {
			paths := map[string]string{}
			for _, sub := range []string{"x", "y"} {
				args := []string{phase, filepath.Join(sub, "a.md")}
				if phase == "audit" {
					args = append(args, "--affected-files", filepath.Join(sub, "code.go"))
				}
				for _, pm := range emittedPrompts(t, dir, noRoundDir(), args...) {
					role := pm["role"].(string)
					want := filepath.Join(sub, ".tp-review", "a", phase+"-inbox", "round-1-"+role+".ndjson")
					assert.Equal(t, want, pm["output_path"], "the role file sits in the spec's own state directory")
					assert.NotContains(t, paths, want)
					paths[want] = role
				}
			}
		})
	}
}

// TestMergeRefusesTheOldRoleFileName: there is no fallback read of the old
// cwd-relative name, which two same-base specs share; --merge names the file
// and where to move it, and merges nothing.
func TestMergeRefusesTheOldRoleFileName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := "review-a-r1-implementer.ndjson"
	require.NoError(t, os.WriteFile(filepath.Join(dir, old), nil, 0o600))
	_, stderr, code := runTP(t, dir, "review", "--merge", old, "-o", "merged.ndjson")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, old, "the refusal names the old file")
	assert.Contains(t, stderr, filepath.Join(".tp-review", "a", "review-inbox", "round-1-implementer.ndjson"), "and where to move it")
	assert.NoFileExists(t, filepath.Join(dir, "merged.ndjson"))
}

// TestRecordRemovesTheRoundsRoleFiles: --record removes the round's role
// files from the inbox once the round is on disk, and the inbox with them.
func TestRecordRemovesTheRoundsRoleFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	files := make([]string, 0, len(prompts))
	for _, pm := range prompts {
		path := filepath.Join(dir, pm["output_path"].(string))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		files = append(files, pm["output_path"].(string))
	}
	merged := filepath.Join(dir, "merged.ndjson")
	_, stderr, code := runTP(t, dir, append(append([]string{"review", "--merge"}, files...), "-o", merged)...)
	require.Equal(t, 0, code, "merge: %s", stderr)
	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", merged)
	require.Equal(t, 0, code, "record: %s", stderr)

	for _, f := range files {
		assert.NoFileExists(t, filepath.Join(dir, f), "--record removes the round's role files")
	}
	assert.NoDirExists(t, filepath.Join(engine.ReviewStateDir(spec), "review-inbox"), "and the emptied inbox")
}

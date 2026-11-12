package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The project root outside any repository (spec/backlog/project-layer-from-
// task-file.md): a .tp/ that exists only because a lock or a run put its
// artifacts there never becomes a project root, and with no project .tp/ and
// no repository at all the first deliberate write decides where the root is.

const looseTask = `{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["# Spec"]}`

// looseProject is a directory outside any repository holding docs/spec.md
// and nothing else: no .tp/ anywhere above it, no .git.
func looseProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	return dir
}

// TestALockNeverMakesAProjectRoot is the reported repro: tp init from the
// directory above docs/ anchors the project there, and the lock its own write
// took beside the task file leaves a docs/.tp/ holding only locks/, which must
// not turn docs/ into a project of its own. Closing the task from the same
// directory is work in this project's tree, not another's.
func TestALockNeverMakesAProjectRoot(t *testing.T) {
	t.Parallel()
	dir := looseProject(t)
	taskFile := filepath.Join("docs", "spec.tasks.json")
	_, stderr, code := runTP(t, dir, "init", filepath.Join("docs", "spec.md"))
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "add", "--file", taskFile, looseTask)
	require.Equal(t, 0, code, "add: %s", stderr)

	assert.FileExists(t, filepath.Join(dir, ".tp", ".gitignore"), "tp init's write anchors the project where it ran")
	assert.NoFileExists(t, filepath.Join(dir, "docs", ".tp", ".gitignore"),
		"a lock outside any repository and project writes no .gitignore, which would make its directory a project")

	_, stderr, code = runTP(t, dir, "done", "--file", taskFile, "t1", "implemented and verified")
	assert.NotContains(t, stderr, "is not this project's", "docs/ is not a project of its own")
	assert.Equal(t, 0, code, "done: %s", stderr)
}

// TestWithNoProjectAndNoRepositoryTheFirstWriteDecides pins the case with
// nothing to discover: a task file made by hand in docs/, no .tp/ anywhere,
// no repository. Each directory is then its own root, so tree work on
// docs/'s task file from the directory above is refused and from docs/
// itself it runs. A lock taken there leaves docs/.tp/ with locks/ only, and
// the root of the directory above stays itself until a deliberate write
// anchors it.
func TestWithNoProjectAndNoRepositoryTheFirstWriteDecides(t *testing.T) {
	t.Parallel()
	dir := looseProject(t)
	_, stderr, code := runTP(t, filepath.Join(dir, "docs"), "init", "spec.md")
	require.Equal(t, 0, code, "init from docs: %s", stderr)
	_, stderr, code = runTP(t, filepath.Join(dir, "docs"), "add", looseTask)
	require.Equal(t, 0, code, "add: %s", stderr)

	_, stderr, code = runTP(t, dir, "done", "--file", filepath.Join("docs", "spec.tasks.json"), "t1", "implemented and verified")
	assert.Equal(t, 3, code, "the first write ran in docs/, so docs/ is the root and the directory above is not: %s", stderr)
	assert.Contains(t, stderr, "is not this project's")

	_, stderr, code = runTP(t, filepath.Join(dir, "docs"), "done", "t1", "implemented and verified")
	assert.Equal(t, 0, code, "from the root the first write chose it runs: %s", stderr)
}

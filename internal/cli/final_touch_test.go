package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestA111LockLeftoverDoesNotAnchorAProject is the upgrade half of the
// side-effect rule: tp 1.11's locked writes put tp's .gitignore into the
// nested .tp/ beside a task file, so after the upgrade docs/.tp/ holds
// .gitignore and locks/ and nothing else. Recognised by its names, whatever
// the .gitignore says, it is not a project root, and closing the task from
// the directory tp init ran in is this project's work.
func TestA111LockLeftoverDoesNotAnchorAProject(t *testing.T) {
	t.Parallel()
	dir := looseProject(t)
	taskFile := filepath.Join("docs", "spec.tasks.json")
	_, stderr, code := runTP(t, dir, "init", filepath.Join("docs", "spec.md"))
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "add", "--file", taskFile, looseTask)
	require.Equal(t, 0, code, "add: %s", stderr)

	// What 1.11 left: its .gitignore (here edited by a user, which must not
	// matter) beside its locks/.
	leftover := filepath.Join(dir, "docs", ".tp")
	require.NoError(t, os.MkdirAll(filepath.Join(leftover, "locks"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(leftover, ".gitignore"), []byte("local.json\nlocks/\n# mine\n*.bak\n"), 0o600))

	_, stderr, code = runTP(t, dir, "done", "--file", taskFile, "t1", "implemented and verified")
	assert.NotContains(t, stderr, "is not this project's", "a side-effect .tp/ is never a project root")
	assert.Equal(t, 0, code, "done: %s", stderr)
}

// TestARunFromASubdirectoryIsFoundFromTheRoot: the run lock and the run state
// are keyed against the project root, so a run started from a subdirectory
// is the run tp run --status reports from the root.
func TestARunFromASubdirectoryIsFoundFromTheRoot(t *testing.T) {
	t.Parallel()
	dir := runExitProject(t, exitBlockedTask)
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	env, _ := seam(t)

	_, stderr, code := runTPEnv(t, sub, env, "run", "--file", filepath.Join(dir, "spec.tasks.json"))
	require.Equal(t, 4, code, "the blocked cycle stops with no units: %s", stderr)

	stdout, stderr, code := runTPEnv(t, dir, env, "run", "--status")
	require.Equal(t, 0, code, "the run started from the subdirectory is found from the root: %s", stderr)
	assert.Contains(t, stdout, "no-units")
}

// TestRunStatusNamesANestedTaskFileByItsPath: with no run state, tp run
// --status names the task file relative to its project root, not by a base
// two task files can share; for a task file of another project the hint says
// where to start the run.
func TestRunStatusNamesANestedTaskFileByItsPath(t *testing.T) {
	t.Parallel()
	here, there, taskFile := twoProjects(t)
	_, stderr, code := runTP(t, here, "run", "--status", "--file", taskFile)
	assert.Equal(t, 3, code)
	resolved, err := filepath.EvalSymlinks(there)
	require.NoError(t, err)
	assert.Contains(t, stderr, "no run state for spec.tasks.json")
	assert.Contains(t, stderr, "start the run from "+resolved)

	nested := filepath.Join(here, "x")
	require.NoError(t, os.MkdirAll(nested, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	_, stderr, code = runTP(t, here, "init", filepath.Join("x", "spec.md"))
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, here, "run", "--status", "--file", filepath.Join("x", "spec.tasks.json"))
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "no run state for "+filepath.Join("x", "spec.tasks.json"))
}

// TestMergeKeepsAUsersOwnFileInTheOldShape: the old-name refusal applies only
// when the role part names a role tp knows, so a file the operator named
// review-spec-r2-all.ndjson is merged as before.
func TestMergeKeepsAUsersOwnFileInTheOldShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mine := "review-spec-r2-all.ndjson"
	require.NoError(t, os.WriteFile(filepath.Join(dir, mine), nil, 0o600))
	_, stderr, code := runTP(t, dir, "review", "--merge", mine, "-o", "merged.ndjson")
	assert.Equal(t, 0, code, "a file whose role part is no role is merged: %s", stderr)
}

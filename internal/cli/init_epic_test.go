package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initBindingDir returns a project holding bindingChain as its graph and a
// spec, spec.md, with no task file yet.
func initBindingDir(t *testing.T) string {
	t.Helper()
	dir := epicGraphDir(t, bindingChain)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	return dir
}

// TestInitEpic_CreatesAFileBoundToTheEpic is spec/1.5.0.md §2: tp init
// <spec> --epic b writes b into the task file it creates.
func TestInitEpic_CreatesAFileBoundToTheEpic(t *testing.T) {
	t.Parallel()
	dir := initBindingDir(t)

	_, stderr, code := runTP(t, dir, "init", "spec.md", "--epic", "b")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "b", readBoundEpic(t, dir))
}

// assertInitBindingRefused runs tp init spec.md --epic id and asserts exit 1,
// stderr naming want, and no task file created.
func assertInitBindingRefused(t *testing.T, id, want string) {
	t.Helper()
	dir := initBindingDir(t)

	_, stderr, code := runTP(t, dir, "init", "spec.md", "--epic", id)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, want)
	_, err := os.Stat(filepath.Join(dir, "spec.tasks.json"))
	assert.True(t, os.IsNotExist(err), "a refused binding creates no task file")
}

// TestInitEpic_AbsentIDIsRefusedAndCreatesNoFile is §2: an init that wrote
// the file before checking the id would leave a file bound to nothing.
func TestInitEpic_AbsentIDIsRefusedAndCreatesNoFile(t *testing.T) {
	t.Parallel()
	assertInitBindingRefused(t, "z", "not in the graph")
}

// TestInitEpic_ConditionIsRefusedAndCreatesNoFile is §2: no task file builds
// a condition.
func TestInitEpic_ConditionIsRefusedAndCreatesNoFile(t *testing.T) {
	t.Parallel()
	assertInitBindingRefused(t, "c", "condition")
}

// TestInitEpic_DoneEpicIsRefusedAndCreatesNoFile is §2: a done epic takes no
// new task file.
func TestInitEpic_DoneEpicIsRefusedAndCreatesNoFile(t *testing.T) {
	t.Parallel()
	assertInitBindingRefused(t, "d", "done")
}

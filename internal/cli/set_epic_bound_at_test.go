package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boundAtOf returns the bound_at of the task file at path, nil when absent.
func boundAtOf(t *testing.T, path string) *time.Time {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var tf struct {
		BoundAt *time.Time `json:"bound_at"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	return tf.BoundAt
}

// setEpic runs tp set --epic id in dir and requires exit 0.
func setEpic(t *testing.T, dir, id string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "set", "--epic", id)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestSetEpic_BoundAtMovesOnlyWhenTheEpicChanges is spec/1.8.0.md §15 and §18
// row 11's binding half: binding an unbound file to a writes bound_at in UTC,
// re-binding it to a writes nothing, and binding it to b moves it. A stamp
// written on every tp set --epic fails the re-bind.
func TestSetEpic_BoundAtMovesOnlyWhenTheEpicChanges(t *testing.T) {
	t.Parallel()
	dir := bindingDir(t, "")
	path := filepath.Join(dir, "spec.tasks.json")
	require.Nil(t, boundAtOf(t, path), "an unbound file carries no bound_at")

	setEpic(t, dir, "a")
	first := boundAtOf(t, path)
	require.NotNil(t, first, "binding writes bound_at")
	assert.Equal(t, time.UTC, first.Location(), "bound_at is in UTC")

	setEpic(t, dir, "a")
	again := boundAtOf(t, path)
	require.NotNil(t, again)
	assert.True(t, again.Equal(*first), "re-binding the same epic leaves bound_at as it was")

	setEpic(t, dir, "b")
	moved := boundAtOf(t, path)
	require.NotNil(t, moved)
	assert.True(t, moved.After(*first), "binding another epic moves bound_at")
}

// TestSetEpic_EmptyRemovesBoundAtWithTheEpic is §15: tp set --epic ""
// removes bound_at together with epic.
func TestSetEpic_EmptyRemovesBoundAtWithTheEpic(t *testing.T) {
	t.Parallel()
	dir := bindingDir(t, "")
	setEpic(t, dir, "a")
	setEpic(t, dir, "")
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"epic"`)
	assert.NotContains(t, string(data), `"bound_at"`)
}

// TestInit_EpicWritesBoundAt is §15: tp init <spec> --epic a writes bound_at
// on the new file, and tp init without --epic writes none.
func TestInit_EpicWritesBoundAt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, bindingChain)
	for _, name := range []string{"bound.md", "free.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("# S\n## 1. Setup\nSetup.\n"), 0o600))
	}
	_, stderr, code := runTP(t, dir, "init", "bound.md", "--epic", "a")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	bound := boundAtOf(t, filepath.Join(dir, "bound.tasks.json"))
	require.NotNil(t, bound, "tp init --epic writes bound_at")
	assert.Equal(t, time.UTC, bound.Location())

	_, stderr, code = runTP(t, dir, "init", "free.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Nil(t, boundAtOf(t, filepath.Join(dir, "free.tasks.json")), "an unbound init writes no bound_at")
}

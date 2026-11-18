package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spec/backlog/worktree-setup.md §1: worktree_setup_cmd is read only from the
// project layer, so the task-level setter refuses it the way it refuses a
// field it cannot write, and points at the layer that can.
func TestWorktreeSetupCmd_TaskLevelSetterRefusesIt(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	stdout, stderr, code := runTPEnv(t, dir, nil, "set", "--workflow", "worktree_setup_cmd=cp -cR ../cache .")
	require.Equal(t, 2, code, "the task layer cannot hold the field: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "--project", "the refusal names the layer that holds it")
}

// The project setter writes it to .tp/config.json, where the driver reads it.
func TestWorktreeSetupCmd_ProjectSetterWritesIt(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	_, stderr, code := runTPEnv(t, dir, nil, "set", "--workflow", "--project", "worktree_setup_cmd=cp -cR ../cache .")
	require.Equal(t, 0, code, "the operator sets it at the project layer: %s", stderr)

	data, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
	require.NoError(t, err)
	var cfg struct {
		Workflow map[string]any `json:"workflow"`
	}
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.Equal(t, "cp -cR ../cache .", cfg.Workflow["worktree_setup_cmd"])
}


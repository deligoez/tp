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


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

// §1: under TP_UNATTENDED a unit cannot set or change the command the driver
// runs in every batch worktree, at either layer.
func TestWorktreeSetupCmd_UnattendedFenceRefusesIt(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	for _, args := range [][]string{
		{"set", "--workflow", "--project", "worktree_setup_cmd=true"},
		{"set", "--workflow", "worktree_setup_cmd=true"},
	} {
		stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn}, args...)
		require.Equal(t, 2, code, "%v must be refused: %s%s", args, stdout, stderr)
		assert.Contains(t, stdout+stderr, "TP_UNATTENDED", "the refusal names the fence: %v", args)
	}
}

// tp config --extract never hoists worktree_setup_cmd: a value a unit wrote
// into task files' workflow blocks must not reach the one layer the driver
// reads it from, attended or not.
func TestWorktreeSetupCmd_ConfigExtractDoesNotHoistIt(t *testing.T) {
	t.Parallel()
	for _, unattended := range []bool{false, true} {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		for _, base := range []string{"a", "b"} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"), []byte("# S\n\n## 1. Setup\n\nDo it.\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, base+".tasks.json"),
				[]byte(`{"version":1,"spec":"`+base+`.md","tasks":[],"workflow":{"worktree_setup_cmd":"touch stray","gate_timeout_seconds":120}}`), 0o600))
		}

		_, stderr, code := runTPFence(t, dir, unattended, "config", "--extract")
		require.Equal(t, 0, code, "config --extract (unattended %v): %s", unattended, stderr)
		cfg, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
		require.NoError(t, err)
		assert.Contains(t, string(cfg), "gate_timeout_seconds", "the hoist ran")
		assert.NotContains(t, string(cfg), "worktree_setup_cmd", "the command is not hoisted (unattended %v)", unattended)
	}
}

// worktreeSetupMoveHint is the refusal a task-file worktree_setup_cmd gets.
const worktreeSetupMoveHint = "read only from the project layer; move it to .tp/config.json"

// validWorktreeSetupProject is a project whose task file validates clean, so
// a refusal its tests see comes from the field they add and nothing else.
func validWorktreeSetupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	importPath := filepath.Join(dir, "seed.json")
	require.NoError(t, os.WriteFile(importPath, []byte(`[`+importWorkflowTask+`]`), 0o600))
	_, stderr, code = runTP(t, dir, "import", importPath, "--spec", "spec.md", "--force")
	require.Equal(t, 0, code, "import: %s", stderr)
	stdout, stderr, code := runTP(t, dir, "validate")
	require.Equal(t, 0, code, "the control validates clean: %s%s", stdout, stderr)
	return dir
}

// tp validate reports a task file's workflow.worktree_setup_cmd as an error:
// the driver never reads that layer, so a value there is a command the
// operator believes runs and does not.
func TestWorktreeSetupCmd_ValidateRefusesItInATaskFile(t *testing.T) {
	t.Parallel()
	dir := validWorktreeSetupProject(t)
	path := filepath.Join(dir, "spec.tasks.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(data, &tf))
	tf["workflow"] = map[string]any{"worktree_setup_cmd": "touch stray"}
	data, err = json.Marshal(tf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	stdout, stderr, code := runTP(t, dir, "validate")
	assert.NotEqual(t, 0, code, "the task file is invalid: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, worktreeSetupMoveHint)
}

// tp import refuses a document whose workflow block carries the field, even
// under --force, which relaxes atomicity but not validation.
func TestWorktreeSetupCmd_ImportRefusesItInTheDocument(t *testing.T) {
	t.Parallel()
	dir := validWorktreeSetupProject(t)
	doc := `{"version":1,"spec":"spec.md","workflow":{"worktree_setup_cmd":"touch stray"},"tasks":[` + importWorkflowTask + `]}`
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))

	stdout, stderr, code := runTP(t, dir, "import", importPath, "--force")
	assert.NotEqual(t, 0, code, "the import is refused: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, worktreeSetupMoveHint)
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "worktree_setup_cmd", "nothing is written")
}

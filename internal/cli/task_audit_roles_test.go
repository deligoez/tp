package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolvedTaskAuditRoles(t *testing.T, dir string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	entry, ok := res["workflow"].(map[string]any)["task_audit_roles"].(map[string]any)
	require.True(t, ok, "tp config --resolved reports task_audit_roles: %s", out)
	return entry
}

func TestTaskAuditRoles_ConfigResolvedShowsDefaultAndLayers(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")

	entry := resolvedTaskAuditRoles(t, dir)
	assert.Equal(t, "default", entry["source"])
	assert.Equal(t, []any{}, entry["value"])

	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", `task_audit_roles=["go-safety"]`)
	require.Equal(t, 0, code, "set --project: %s", stderr)
	entry = resolvedTaskAuditRoles(t, dir)
	assert.Equal(t, "project", entry["source"])
	assert.Equal(t, []any{"go-safety"}, entry["value"])

	_, stderr, code = runTP(t, dir, "set", "--workflow", `task_audit_roles=[]`)
	require.Equal(t, 0, code, "set: %s", stderr)
	entry = resolvedTaskAuditRoles(t, dir)
	assert.Equal(t, "override", entry["source"])
	assert.Equal(t, []any{}, entry["value"], "a present empty task layer wins")
}

func TestTaskAuditRoles_SetRefusesSpecCoverageAndNonArrays(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	for _, args := range [][]string{
		{"set", "--workflow", `task_audit_roles=["spec-coverage"]`},
		{"set", "--workflow", "--project", `task_audit_roles=["go-safety","spec-coverage"]`},
	} {
		_, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 2, code, "%v: %s", args, stderr)
		assert.Contains(t, stderr, "only code roles may audit a task", "%v", args)
	}
	for _, args := range [][]string{
		{"set", "--workflow", `task_audit_roles=go-safety`},
		{"set", "--workflow", `task_audit_roles=["go-safety","go-safety"]`},
		{"set", "--workflow", "--project", `task_audit_roles={"a":1}`},
	} {
		_, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 2, code, "%v: %s", args, stderr)
		assert.Contains(t, stderr, "task_audit_roles", "%v", args)
	}
	entry := resolvedTaskAuditRoles(t, dir)
	assert.Equal(t, "default", entry["source"], "no refused write landed")
}

func TestTaskAuditRoles_DoneStampsTheResolvedRoles(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "set", "--workflow", `task_audit_roles=["go-safety","security"]`)
	require.Equal(t, 0, code, "set: %s", stderr)

	_, stderr, code = runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	assert.Equal(t, []any{"go-safety", "security"}, taskState(t, dir, "t1")["task_audit_owed"])

	_, stderr, code = runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code, "reopen: %s", stderr)
	_, present := taskState(t, dir, "t1")["task_audit_owed"]
	assert.False(t, present, "reopen clears the stamp")
}

func TestTaskAuditRoles_CloseStamps(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "set", "--workflow", `task_audit_roles=["go-safety"]`)
	require.Equal(t, 0, code, "set: %s", stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim: %s", stderr)
	_, stderr, code = runTP(t, dir, "close", "t1", "--skip-gate", "no gate here", "t1 acceptance met")
	require.Equal(t, 0, code, "close: %s", stderr)
	assert.Equal(t, []any{"go-safety"}, taskState(t, dir, "t1")["task_audit_owed"])
}

func TestTaskAuditRoles_DoneWithNoRolesWritesNoKey(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "task_audit_owed")
}

func TestTaskAuditRoles_MultiAndBatchDoneStamp(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2", "t3")
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", `task_audit_roles=["go-safety"]`)
	require.Equal(t, 0, code, "set --project: %s", stderr)

	_, stderr, code = runTP(t, dir, "done", "t1", "t2", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met; t2 acceptance met")
	require.Equal(t, 0, code, "multi done: %s", stderr)
	assert.Equal(t, []any{"go-safety"}, taskState(t, dir, "t1")["task_audit_owed"])
	assert.Equal(t, []any{"go-safety"}, taskState(t, dir, "t2")["task_audit_owed"])

	res, stderr, code := runBatch(t, dir, `{"id":"t3","reason":"t3 acceptance met","gate_passed":true,"commit":"ccc"}`)
	require.Equal(t, 0, code, "batch: %s", stderr)
	assert.EqualValues(t, 1, res["closed"])
	assert.Equal(t, []any{"go-safety"}, taskState(t, dir, "t3")["task_audit_owed"])
}

// The stamp is tp done's record of the debt: tp set, tp add and tp import
// cannot write it, so a hand-written task can neither forge nor erase it
// through tp.
func TestTaskAuditOwed_IsManaged(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")

	_, stderr, code := runTP(t, dir, "set", "t1", `task_audit_owed=["go-safety"]`)
	assert.Equal(t, 2, code, "set: %s", stderr)
	assert.Contains(t, stderr, "managed")

	_, stderr, code = runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 done","source_sections":["s1"],"task_audit_owed":["go-safety"]}`)
	assert.Equal(t, 1, code, "add: %s", stderr)
	assert.Contains(t, stderr, "task_audit_owed")

	other := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(other, "other.md"), []byte("# Other\n\n## 1. A\n\nText.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(other, "plan.json"), []byte(`[{"id":"x1","title":"X","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":["## 1. A"],"task_audit_owed":["go-safety"]}]`), 0o600))
	_, stderr, code = runTP(t, other, "import", "plan.json", "--spec", "other.md", "--force")
	assert.Equal(t, 1, code, "import: %s", stderr)
	assert.Contains(t, stderr, "task_audit_owed")
	_, err := os.Stat(filepath.Join(other, "other.tasks.json"))
	assert.True(t, os.IsNotExist(err), "the refused import wrote nothing")
}

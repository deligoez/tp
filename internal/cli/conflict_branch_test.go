package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setConflictBranch writes conflict_branch on task id the way tp run's solo
// reopen leaves it after a unit's commits conflicted with the main branch.
func setConflictBranch(t *testing.T, dir, id, branch string) {
	t.Helper()
	path := filepath.Join(dir, "spec.tasks.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(data, &tf))
	for _, raw := range tf["tasks"].([]any) {
		if task := raw.(map[string]any); task["id"] == id {
			task["conflict_branch"] = branch
		}
	}
	data, err = json.MarshalIndent(tf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// TestConflictBranch_TheBriefNamesItAndTheCloseClearsIt: a task tp run
// reopened over a conflict carries its unit branch into the brief tp next
// and tp brief print, so the next unit resolves the conflict rather than
// redoing the task, and closing the task clears it
// (spec/backlog/worktree-dispatch.md §2.2).
func TestConflictBranch_TheBriefNamesItAndTheCloseClearsIt(t *testing.T) {
	t.Parallel()
	const branch = "tp-unit/abc123/r1/t1"
	dir := setupProject(t)
	addTask(t, dir, `{"id":"t1","title":"T1","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["# Spec"]}`)
	setConflictBranch(t, dir, "t1", branch)

	stdout, stderr, code := runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	var brief struct {
		Task struct {
			ConflictBranch string `json:"conflict_branch"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Equal(t, branch, brief.Task.ConflictBranch)

	_, stderr, code = runTP(t, dir, "done", "t1", "T1 is done and its conflict resolved, verified by its tests")
	require.Equal(t, 0, code, stderr)
	assert.NotContains(t, taskState(t, dir, "t1"), "conflict_branch", "the close clears it")
}

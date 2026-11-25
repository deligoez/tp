package engine

import (
	"encoding/json"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildBrief_AConflictBranchIsResolvedNotRedone: a task whose earlier
// unit's commits did not apply to the main branch carries that branch into
// its brief, as .task.conflict_branch and as a section telling the unit to
// bring the branch onto the current branch rather than start over
// (spec/backlog/worktree-dispatch.md §2.2). A task without one carries
// neither.
func TestBuildBrief_AConflictBranchIsResolvedNotRedone(t *testing.T) {
	const branch = "tp-unit/abc123/r1/unit"
	tasks := []model.Task{
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, Acceptance: "done", ConflictBranch: branch},
		{ID: "bare", Title: "Bare", Status: model.StatusOpen, Acceptance: "done"},
	}
	tf := &model.TaskFile{Tasks: tasks}

	b, err := BuildBrief(tf, &tasks[0], "", CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	raw, err := json.Marshal(b)
	require.NoError(t, err)
	var got struct {
		Task map[string]any `json:"task"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, branch, got.Task["conflict_branch"])
	text := b.Text()
	assert.Contains(t, text, "## Earlier work")
	assert.Contains(t, text, "git cherry-pick HEAD.."+branch)
	assert.Contains(t, text, "do not start the task again")

	b, err = BuildBrief(tf, &tasks[1], "", CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	raw, err = json.Marshal(b)
	require.NoError(t, err)
	got.Task = nil
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.NotContains(t, got.Task, "conflict_branch")
	assert.NotContains(t, b.Text(), "## Earlier work")
}

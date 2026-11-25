package engine

import (
	"encoding/json"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildBrief_CarriesTheTasksTouches: a task's touches globs reach the
// brief, as .task.touches and as a list in the text brief; a task without
// touches carries no key.
func TestBuildBrief_CarriesTheTasksTouches(t *testing.T) {
	tasks := []model.Task{
		{ID: "unit", Title: "Unit", Status: model.StatusOpen, Acceptance: "done", Touches: []string{"internal/x/**", "docs/a.md"}},
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
	assert.Equal(t, []any{"internal/x/**", "docs/a.md"}, got.Task["touches"])
	assert.Contains(t, b.Text(), "\ntouches:\n- internal/x/**\n- docs/a.md\n")

	b, err = BuildBrief(tf, &tasks[1], "", CommitStrategyBuiltin, "", 0, false, false)
	require.NoError(t, err)
	raw, err = json.Marshal(b)
	require.NoError(t, err)
	got.Task = nil
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.NotContains(t, got.Task, "touches")
	assert.NotContains(t, b.Text(), "touches:")
}

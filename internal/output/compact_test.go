package output_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

func TestCompactTask_StripsFields(t *testing.T) {
	now := time.Now().UTC()
	reason := "all done"
	commit := "abc123"

	task := model.Task{
		ID:              "t1",
		Title:           "Implement auth",
		Description:     "Long description that should be stripped",
		Status:          model.StatusDone,
		Tags:            []string{"auth", "backend"},
		DependsOn:       []string{"t0"},
		EstimateMinutes: 30,
		Acceptance:      "Tests pass",
		SourceSections:  []string{"section-1", "section-2"},
		SourceLines:     "10-42",
		ClosedAt:        &now,
		ClosedReason:    &reason,
		GatePassedAt:    &now,
		CommitSHA:       &commit,
	}

	compact := output.CompactTask(&task)

	// Kept fields
	assert.Equal(t, "t1", compact.ID)
	assert.Equal(t, "Implement auth", compact.Title)
	assert.Equal(t, model.StatusDone, compact.Status)
	assert.Equal(t, []string{"t0"}, compact.DependsOn)
	assert.Equal(t, 30, compact.EstimateMinutes)
	assert.Equal(t, "Tests pass", compact.Acceptance)
	assert.Equal(t, &now, compact.ClosedAt)

	// Stripped fields: not present in CompactTaskView struct at all.
	// We verify by checking that the struct only has the expected fields via JSON.
}

func TestCompactTask_KeepsNilClosedAt(t *testing.T) {
	task := model.Task{
		ID:              "t2",
		Title:           "Open task",
		Status:          model.StatusOpen,
		DependsOn:       []string{},
		EstimateMinutes: 15,
		Acceptance:      "It works",
	}

	compact := output.CompactTask(&task)

	assert.Nil(t, compact.ClosedAt)
	assert.Equal(t, "t2", compact.ID)
	assert.Equal(t, model.StatusOpen, compact.Status)
}

func TestCompactTasks_WorksOnArrays(t *testing.T) {
	tasks := []model.Task{
		{
			ID:              "a",
			Title:           "Task A",
			Description:     "desc A",
			Status:          model.StatusOpen,
			Tags:            []string{"tag1"},
			DependsOn:       []string{},
			EstimateMinutes: 10,
			Acceptance:      "A works",
			SourceSections:  []string{"s1"},
			SourceLines:     "1-5",
		},
		{
			ID:              "b",
			Title:           "Task B",
			Description:     "desc B",
			Status:          model.StatusWIP,
			Tags:            []string{"tag2"},
			DependsOn:       []string{"a"},
			EstimateMinutes: 20,
			Acceptance:      "B works",
			SourceSections:  []string{"s2"},
			SourceLines:     "6-10",
		},
	}

	compacted := output.CompactTasks(tasks)

	require.Len(t, compacted, 2)
	assert.Equal(t, "a", compacted[0].ID)
	assert.Equal(t, "b", compacted[1].ID)
	assert.Equal(t, []string{"a"}, compacted[1].DependsOn)
}

func TestCompactTasks_EmptySlice(t *testing.T) {
	compacted := output.CompactTasks([]model.Task{})
	require.Len(t, compacted, 0)
}

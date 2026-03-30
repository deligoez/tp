package output

import (
	"time"

	"github.com/deligoez/tp/internal/model"
)

// CompactTaskView is a stripped-down view of a Task for token efficiency.
// Fields like description, source_sections, source_lines, tags, and closed_reason are omitted.
type CompactTaskView struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	DependsOn       []string   `json:"depends_on"`
	EstimateMinutes int        `json:"estimate_minutes"`
	Acceptance      string     `json:"acceptance"`
	ClosedAt        *time.Time `json:"closed_at"`
}

// CompactTask creates a CompactTaskView from a Task.
func CompactTask(t *model.Task) CompactTaskView {
	return CompactTaskView{
		ID:              t.ID,
		Title:           t.Title,
		Status:          t.Status,
		DependsOn:       t.DependsOn,
		EstimateMinutes: t.EstimateMinutes,
		Acceptance:      t.Acceptance,
		ClosedAt:        t.ClosedAt,
	}
}

// CompactTasks creates a slice of CompactTaskView from a slice of Tasks.
func CompactTasks(tasks []model.Task) []CompactTaskView {
	result := make([]CompactTaskView, len(tasks))
	for i := range tasks {
		result[i] = CompactTask(&tasks[i])
	}
	return result
}

package engine

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

func TestComputeParallelismLevels(t *testing.T) {
	tests := []struct {
		name       string
		tasks      []model.Task
		wantLevels int
		check      func(t *testing.T, levels []ParallelLevel)
	}{
		{
			name:       "no tasks",
			tasks:      nil,
			wantLevels: 0,
			check: func(t *testing.T, levels []ParallelLevel) {
				assert.Empty(t, levels)
			},
		},
		{
			name: "single task no deps",
			tasks: []model.Task{
				{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
			},
			wantLevels: 1,
			check: func(t *testing.T, levels []ParallelLevel) {
				require.Len(t, levels, 1)
				assert.Equal(t, 0, levels[0].Level)
				assert.Equal(t, []string{"A"}, levels[0].Tasks)
			},
		},
		{
			name: "linear chain A then B then C",
			tasks: []model.Task{
				{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
				{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{"A"}},
				{ID: "C", Title: "C", Status: "open", Acceptance: "c", EstimateMinutes: 5, DependsOn: []string{"B"}},
			},
			wantLevels: 3,
			check: func(t *testing.T, levels []ParallelLevel) {
				require.Len(t, levels, 3)
				assert.Equal(t, []string{"A"}, levels[0].Tasks)
				assert.Equal(t, []string{"B"}, levels[1].Tasks)
				assert.Equal(t, []string{"C"}, levels[2].Tasks)
			},
		},
		{
			name: "wide graph three independent tasks",
			tasks: []model.Task{
				{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
				{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{}},
				{ID: "C", Title: "C", Status: "open", Acceptance: "c", EstimateMinutes: 5, DependsOn: []string{}},
			},
			wantLevels: 1,
			check: func(t *testing.T, levels []ParallelLevel) {
				require.Len(t, levels, 1)
				assert.Equal(t, 0, levels[0].Level)
				assert.Len(t, levels[0].Tasks, 3)
			},
		},
		{
			name: "diamond A to B and C then D",
			tasks: []model.Task{
				{ID: "A", Title: "A", Status: "open", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
				{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{"A"}},
				{ID: "C", Title: "C", Status: "open", Acceptance: "c", EstimateMinutes: 5, DependsOn: []string{"A"}},
				{ID: "D", Title: "D", Status: "open", Acceptance: "d", EstimateMinutes: 5, DependsOn: []string{"B", "C"}},
			},
			wantLevels: 3,
			check: func(t *testing.T, levels []ParallelLevel) {
				require.Len(t, levels, 3)
				assert.Equal(t, []string{"A"}, levels[0].Tasks)
				// B and C at level 1
				sort.Strings(levels[1].Tasks)
				assert.Equal(t, []string{"B", "C"}, levels[1].Tasks)
				assert.Equal(t, []string{"D"}, levels[2].Tasks)
			},
		},
		{
			name: "done tasks excluded",
			tasks: []model.Task{
				{ID: "A", Title: "A", Status: "done", Acceptance: "a", EstimateMinutes: 5, DependsOn: []string{}},
				{ID: "B", Title: "B", Status: "open", Acceptance: "b", EstimateMinutes: 5, DependsOn: []string{"A"}},
			},
			wantLevels: 1,
			check: func(t *testing.T, levels []ParallelLevel) {
				require.Len(t, levels, 1)
				assert.Equal(t, 0, levels[0].Level)
				assert.Equal(t, []string{"B"}, levels[0].Tasks)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels := ComputeParallelismLevels(tt.tasks)
			if tt.wantLevels == 0 {
				assert.Empty(t, levels)
			} else {
				assert.Len(t, levels, tt.wantLevels)
			}
			if tt.check != nil {
				tt.check(t, levels)
			}
		})
	}
}

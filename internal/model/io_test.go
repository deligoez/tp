package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadWriteTaskFile_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tf   TaskFile
	}{
		{
			name: "full task file",
			tf: TaskFile{
				Version:   1,
				Spec:      "spec.md",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Workflow: Workflow{
					QualityGate:    "tests-pass",
					CommitStrategy: "squash",
				},
				Coverage: Coverage{
					TotalSections:  10,
					MappedSections: 8,
					ContextOnly:    []string{"intro"},
					Unmapped:       []string{"appendix"},
				},
				Tasks: []Task{
					{
						ID:              "task-1",
						Title:           "First task",
						Description:     "Do something",
						Status:          StatusOpen,
						Tags:            []string{"backend"},
						DependsOn:       []string{},
						EstimateMinutes: 30,
						Acceptance:      "Tests pass",
						SourceSections:  []string{"section-1"},
					},
					{
						ID:              "task-2",
						Title:           "Second task",
						Status:          StatusWIP,
						DependsOn:       []string{"task-1"},
						EstimateMinutes: 15,
						Acceptance:      "Lint passes",
						SourceSections:  []string{"section-2"},
					},
				},
			},
		},
		{
			name: "empty tasks",
			tf: TaskFile{
				Version:   1,
				Spec:      "empty.md",
				CreatedAt: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
				Workflow:  Workflow{},
				Coverage:  Coverage{ContextOnly: []string{}, Unmapped: []string{}},
				Tasks:     []Task{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.tasks.json")

			err := WriteTaskFile(path, &tt.tf)
			require.NoError(t, err)

			got, err := ReadTaskFile(path)
			require.NoError(t, err)

			assert.Equal(t, tt.tf.Version, got.Version)
			assert.Equal(t, tt.tf.Spec, got.Spec)
			assert.Equal(t, tt.tf.CreatedAt, got.CreatedAt)
			// UpdatedAt is set by WriteTaskFile to time.Now(), so just check it's recent
			assert.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, 5*time.Second)
			assert.Equal(t, tt.tf.Workflow, got.Workflow)
			assert.Equal(t, tt.tf.Coverage, got.Coverage)
			assert.Equal(t, len(tt.tf.Tasks), len(got.Tasks))

			for i := range tt.tf.Tasks {
				assert.Equal(t, tt.tf.Tasks[i].ID, got.Tasks[i].ID)
				assert.Equal(t, tt.tf.Tasks[i].Title, got.Tasks[i].Title)
				assert.Equal(t, tt.tf.Tasks[i].Status, got.Tasks[i].Status)
			}
		})
	}
}

func TestReadTaskFile_NonexistentPath(t *testing.T) {
	_, err := ReadTaskFile("/nonexistent/path/tasks.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read task file")
}

func TestReadTaskFile_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"garbage", "this is not json"},
		{"truncated", `{"version": 1, "spec":`},
		{"wrong type", `["not", "an", "object"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "bad.tasks.json")
			err := os.WriteFile(path, []byte(tt.content), 0o600)
			require.NoError(t, err)

			_, err = ReadTaskFile(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "parse task file")
		})
	}
}

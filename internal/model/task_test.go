package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"open to wip", StatusOpen, StatusWIP, true},
		{"wip to done", StatusWIP, StatusDone, true},
		{"done to open", StatusDone, StatusOpen, true},
		{"open to done invalid", StatusOpen, StatusDone, false},
		{"wip to open invalid", StatusWIP, StatusOpen, false},
		{"done to wip invalid", StatusDone, StatusWIP, false},
		{"open to open same", StatusOpen, StatusOpen, false},
		{"wip to wip same", StatusWIP, StatusWIP, false},
		{"done to done same", StatusDone, StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidTransition(tt.from, tt.to))
		})
	}
}

func TestDepsAlias(t *testing.T) {
	t.Run("deps populates depends_on", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","deps":["a","b"],"estimate_minutes":5,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Equal(t, []string{"a", "b"}, task.DependsOn)
	})

	t.Run("depends_on still works", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","depends_on":["x"],"estimate_minutes":5,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Equal(t, []string{"x"}, task.DependsOn)
	})

	t.Run("depends_on takes precedence over deps", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","depends_on":["x"],"deps":["y"],"estimate_minutes":5,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Equal(t, []string{"x"}, task.DependsOn)
	})

	t.Run("neither present gives nil", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","estimate_minutes":5,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Nil(t, task.DependsOn)
	})
}

func TestEstimationMinutesAlias(t *testing.T) {
	t.Run("estimation_minutes populates estimate_minutes", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","estimation_minutes":5,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Equal(t, 5, task.EstimateMinutes)
	})

	t.Run("estimate_minutes still works", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","estimate_minutes":10,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Equal(t, 10, task.EstimateMinutes)
	})

	t.Run("estimate_minutes takes precedence over estimation_minutes", func(t *testing.T) {
		data := `{"id":"t1","title":"T","status":"open","estimate_minutes":10,"estimation_minutes":5,"acceptance":"ok","source_sections":[]}`
		var task Task
		require.NoError(t, json.Unmarshal([]byte(data), &task))
		assert.Equal(t, 10, task.EstimateMinutes)
	})
}

func TestValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"open is valid", "open", true},
		{"wip is valid", "wip", true},
		{"done is valid", "done", true},
		{"blocked is invalid", "blocked", false},
		{"deferred is invalid", "deferred", false},
		{"empty is invalid", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidStatus(tt.status))
		})
	}
}

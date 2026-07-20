package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

func TestWorkflowChecks_Validation(t *testing.T) {
	valid := model.Check{Class: "ok-check", Cmd: "true"}

	t.Run("invalid slug rejected with index", func(t *testing.T) {
		err := ValidateChecks([]model.Check{valid, {Class: "Bad_Slug", Cmd: "true"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checks[1]")
		assert.Contains(t, err.Error(), "must match")
	})

	t.Run("duplicate class rejected with index", func(t *testing.T) {
		err := ValidateChecks([]model.Check{valid, {Class: "ok-check", Cmd: "other"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checks[1]")
		assert.Contains(t, err.Error(), "duplicate class")
	})

	t.Run("empty cmd rejected with index", func(t *testing.T) {
		err := ValidateChecks([]model.Check{{Class: "empty-cmd", Cmd: "   "}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checks[0]")
		assert.Contains(t, err.Error(), "cmd must be non-empty")
	})

	t.Run("valid list passes", func(t *testing.T) {
		assert.NoError(t, ValidateChecks([]model.Check{valid, {Class: "second-check", Cmd: "echo hi"}}))
	})
}

func TestValidate_GateTimeoutRange(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\ncontent\n"), 0o600))

	tf := &model.TaskFile{
		Version: 1,
		Spec:    "spec.md",
		Workflow: model.Workflow{
			ReviewCleanRounds:  2,
			AuditCleanRounds:   2,
			GateTimeoutSeconds: 9999, // out of 30-3600
			Checks:             []model.Check{},
		},
		Tasks: []model.Task{
			{ID: "t1", Title: "T", Status: model.StatusOpen, EstimateMinutes: 5, Acceptance: "ok", SourceLines: "1-2"},
		},
	}

	result := Validate(tf, specPath, true, false)
	found := false
	for _, f := range result.Findings {
		if f.Severity == "warning" && f.Rule == "schema" &&
			strings.Contains(f.Message, "gate_timeout_seconds") && strings.Contains(f.Message, "out of range") {
			found = true
		}
	}
	assert.True(t, found, "out-of-range gate_timeout_seconds produces a validate warning")

	assert.Equal(t, 600, tf.Workflow.EffectiveGateTimeoutSeconds(), "effective timeout falls back to 600")
}

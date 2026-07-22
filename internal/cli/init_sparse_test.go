package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInit_SparseWorkflowBlock covers §10.1: tp init writes a sparse workflow
// block with no default materialization; the quality-gate and commit-strategy
// flags each write exactly their field, and both flags compose.
func TestInit_SparseWorkflowBlock(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		expect map[string]any
	}{
		{"no flags writes empty object", nil, map[string]any{}},
		{"quality gate only", []string{"--quality-gate", "go test ./..."}, map[string]any{"quality_gate": "go test ./..."}},
		{"commit strategy only", []string{"--commit-strategy", "squash"}, map[string]any{"commit_strategy": "squash"}},
		{"both flags compose", []string{"--quality-gate", "go test ./...", "--commit-strategy", "squash"}, map[string]any{"quality_gate": "go test ./...", "commit_strategy": "squash"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
			args := append([]string{"init", "spec.md"}, tc.args...)
			_, stderr, code := runTP(t, dir, args...)
			require.Equal(t, 0, code, "init failed: %s", stderr)

			data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
			require.NoError(t, err)
			var tf map[string]any
			require.NoError(t, json.Unmarshal(data, &tf))
			assert.Equal(t, tc.expect, tf["workflow"], "the workflow block contains exactly the flagged fields, no materialized defaults")
		})
	}
}

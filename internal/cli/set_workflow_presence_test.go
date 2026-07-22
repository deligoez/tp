package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetWorkflow_ErrorTaxonomyAndPresence covers §6.1: tp set --workflow writes
// only the named field, and its error taxonomy — out-of-range or non-parseable
// values are validation errors (exit 1), unknown or non-settable fields are
// usage errors (exit 2), and a rejected set leaves the file unmodified.
func TestSetWorkflow_ErrorTaxonomyAndPresence(t *testing.T) {
	setup := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
			[]byte(`{"version":1,"spec":"spec.md","tasks":[],"workflow":{}}`), 0o600))
		return dir
	}
	readWorkflow := func(t *testing.T, dir string) map[string]any {
		data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
		require.NoError(t, err)
		var tf map[string]any
		require.NoError(t, json.Unmarshal(data, &tf))
		return tf["workflow"].(map[string]any)
	}

	t.Run("successful set writes only that field", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := runTP(t, dir, "set", "--workflow", "review_max_rounds=4")
		require.Equal(t, 0, code, "set failed: %s", stderr)
		assert.Equal(t, map[string]any{"review_max_rounds": float64(4)}, readWorkflow(t, dir), "only the named field is present, no siblings materialized")
	})

	t.Run("out-of-range value exits 1 and leaves the file unmodified", func(t *testing.T) {
		dir := setup(t)
		_, _, code := runTP(t, dir, "set", "--workflow", "review_clean_rounds=0")
		assert.Equal(t, 1, code, "out-of-range is a validation error")
		assert.Empty(t, readWorkflow(t, dir), "the workflow block is unchanged")
	})

	t.Run("non-parseable value exits 1", func(t *testing.T) {
		dir := setup(t)
		_, _, code := runTP(t, dir, "set", "--workflow", "review_clean_rounds=abc")
		assert.Equal(t, 1, code, "a non-integer is a validation error")
		assert.Empty(t, readWorkflow(t, dir))
	})

	t.Run("non-settable field exits 2", func(t *testing.T) {
		dir := setup(t)
		_, _, code := runTP(t, dir, "set", "--workflow", "quality_gate=go test ./...")
		assert.Equal(t, 2, code, "quality_gate is init-only, a usage error")
		assert.Empty(t, readWorkflow(t, dir))
	})

	t.Run("unknown field exits 2", func(t *testing.T) {
		dir := setup(t)
		_, _, code := runTP(t, dir, "set", "--workflow", "bogus=1")
		assert.Equal(t, 2, code, "an unknown field is a usage error")
		assert.Empty(t, readWorkflow(t, dir))
	})
}

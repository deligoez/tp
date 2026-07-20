package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegressionPrompt_AutoInclusion: appended only when diff or fixed
// findings exist; standalone mode requires inputs; standalone never writes
// state.
func TestRegressionPrompt_AutoInclusion(t *testing.T) {
	t.Run("appended on diff at round 2 with process-first instruction", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\noriginal\n"), 0o600))
		_, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		_, _, code = recordRound(t, dir, "")
		require.Equal(t, 0, code)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nchanged\n"), 0o600))

		stdout, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		prompts := out["prompts"].([]any)
		require.Len(t, prompts, 4)
		last := prompts[3].(map[string]any)
		assert.Equal(t, "regression", last["role"], "regression appended as the 4th entry")

		loop := out["review_loop"].(map[string]any)
		assert.Contains(t, loop["instruction"], "Process the regression prompt first")
		assert.Contains(t, loop["instruction"], "uncounted delta pass", "delta-pass usage documented")
	})

	t.Run("not appended without diff or fixed findings", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nstable\n"), 0o600))
		_, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		_, _, code = recordRound(t, dir, "")
		require.Equal(t, 0, code)

		// Spec unchanged, no fixed findings: round 2 stays 3 prompts
		stdout, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		assert.Len(t, out["prompts"].([]any), 3)
		loop := out["review_loop"].(map[string]any)
		assert.NotContains(t, loop["instruction"], "regression prompt")
	})

	t.Run("appended on fixed finding even without diff", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nstable\n"), 0o600))
		_, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		_, _, code = recordRound(t, dir,
			`{"severity":"high","category":"consistency","location":"L1","finding":"was broken","suggestion":"s"}`+"\n")
		require.Equal(t, 0, code)
		roundFile := filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson")
		_, _, code = runTP(t, dir, "review", "--resolve", roundFile, "0", "fixed", "fixed in place")
		require.Equal(t, 0, code)

		// Spec unchanged (snapshot identical) but a fixed finding exists.
		// Note: resolving edits the round file, not the spec, so the diff is
		// empty and only the fixed-findings leg triggers inclusion.
		stdout, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		prompts := out["prompts"].([]any)
		require.Len(t, prompts, 4)
		text := prompts[3].(map[string]any)["prompt"].(string)
		assert.Contains(t, text, "was broken — fixed in place")
	})

	t.Run("standalone mode requires inputs", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		_, _, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression")
		assert.Equal(t, 2, code, "no state and no explicit inputs is a usage error")
	})

	t.Run("standalone never writes state", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\noriginal\n"), 0o600))
		_, _, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code)
		_, _, code = recordRound(t, dir, "")
		require.Equal(t, 0, code)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nchanged\n"), 0o600))

		stateDir := filepath.Join(dir, ".tp-review", "spec")
		before, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
		require.NoError(t, err)
		entriesBefore, err := os.ReadDir(stateDir)
		require.NoError(t, err)

		_, _, code = runTP(t, dir, "review", "spec.md", "--perspective", "regression")
		require.Equal(t, 0, code)

		after, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
		require.NoError(t, err)
		assert.Equal(t, before, after, "state.json untouched")
		entriesAfter, err := os.ReadDir(stateDir)
		require.NoError(t, err)
		assert.Equal(t, len(entriesBefore), len(entriesAfter), "no snapshot or round file written")
	})
}

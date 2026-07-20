package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewPromptsOf(t *testing.T, stdout string) []string {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	raw := out["prompts"].([]any)
	prompts := make([]string, 0, len(raw))
	for _, p := range raw {
		prompts = append(prompts, p.(map[string]any)["prompt"].(string))
	}
	return prompts
}

func TestReviewAutoDiff_SnapshotBaseline(t *testing.T) {
	dir := t.TempDir()
	spec1 := "# Spec\n## 1. Stable\nstable content\n## 2. Volatile\noriginal text\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec1), 0o600))

	// Round 1: generates snapshot; no diff block (no earlier snapshot)
	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	for _, p := range reviewPromptsOf(t, stdout) {
		assert.NotContains(t, p, "Changed sections since", "round 1 has no diff block")
	}
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)

	// Edit the spec, round 2: diff block against the round-1 snapshot
	spec2 := "# Spec\n## 1. Stable\nstable content\n## 2. Volatile\nrewritten text\n## 3. Brand New\nnew section body\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec2), 0o600))

	stdout, _, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	prompts := reviewPromptsOf(t, stdout)
	require.Len(t, prompts, 3)
	for _, p := range prompts {
		assert.Contains(t, p, "Changed sections since round 1", "every prompt carries the block")
		assert.Contains(t, p, "2. Volatile")
		assert.Contains(t, p, "rewritten text", "per-section new content included")
		assert.Contains(t, p, "3. Brand New")
		assert.NotContains(t, strings.SplitN(p, "Changed sections since", 2)[1], "1. Stable", "unchanged sections stay out of the block")
	}
}

func TestReviewAutoDiff_SectionLineCap(t *testing.T) {
	dir := t.TempDir()
	long := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		long = append(long, "filler line content")
	}
	spec1 := "# Spec\n## 1. Big\noriginal\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec1), 0o600))
	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)

	spec2 := "# Spec\n## 1. Big\n" + strings.Join(long, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec2), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	prompts := reviewPromptsOf(t, stdout)
	assert.Contains(t, prompts[0], "[...section truncated at 40 lines]")
}

func TestReviewAutoDiff_DiffFromForcesAtRound1(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.md"), []byte("# Spec\n## 1. A\nold\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nnew\n"), 0o600))

	// No state, round 1, explicit --diff-from: block forced
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state", "--diff-from", "base.md")
	require.Equal(t, 0, code, "diff-from no longer requires round >= 2")
	prompts := reviewPromptsOf(t, stdout)
	assert.Contains(t, prompts[0], "Changed sections since baseline base.md")
}

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolvedConfig_ClampedFieldReportsResolvedSource covers §3.4/§10.8: a
// present but out-of-range task override is clamped to absent, so
// tp config --resolved reports the resolved value with source "project" (or
// "default"), never "override".
func TestResolvedConfig_ClampedFieldReportsResolvedSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_clean_rounds":3}}`), 0o600))
	// The task file sets an out-of-range review_clean_rounds (0, outside 1-10).
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_clean_rounds":0}}`), 0o600))

	t.Chdir(root)
	wf, override := resolveConfigWorkflow()
	workflow := resolvedConfig(&wf, override)["workflow"].(map[string]any)
	field := workflow["review_clean_rounds"].(map[string]any)

	assert.Equal(t, "project", field["source"], "a clamped out-of-range override is attributed to the project layer, not override")
	assert.Equal(t, 3, field["value"], "the resolved value is the project value, not the out-of-range file value")
}

// TestResolvedConfig_CommitStrategySource covers §10.7: tp config --resolved
// attributes a task-file commit_strategy to source "override".
func TestResolvedConfig_CommitStrategySource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"commit_strategy":"squash"}}`), 0o600))

	t.Chdir(root)
	wf, override := resolveConfigWorkflow()
	workflow := resolvedConfig(&wf, override)["workflow"].(map[string]any)
	field := workflow["commit_strategy"].(map[string]any)

	assert.Equal(t, "override", field["source"], "a task-file commit_strategy reports source override")
	assert.Equal(t, "builtin", field["value"], "an unrecognized commit_strategy resolves to builtin (§5.2)")
}

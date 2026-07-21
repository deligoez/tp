package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveConfigWorkflow_LayersProjectUnderTaskFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":8,"gate_timeout_seconds":1200}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":2}}`), 0o600))

	t.Chdir(root)
	wf, _ := resolveConfigWorkflow()
	assert.Equal(t, 2, wf.ReviewMaxRounds, "task-file override wins")
	assert.Equal(t, 1200, wf.GateTimeoutSeconds, "project value inherited where the task file is silent")
}

func TestResolveConfigWorkflow_NoTaskFileProjectAlone(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":5}}`), 0o600))

	t.Chdir(root)
	wf, override := resolveConfigWorkflow()
	assert.True(t, override.IsEmpty(), "no task file means no per-task overrides")
	assert.Equal(t, 5, wf.ReviewMaxRounds, "project layer alone")
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default where the project is silent")
}

func TestResolvedConfig_SourceLabels(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":8}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_clean_rounds":3}}`), 0o600))

	t.Chdir(root)
	wf, override := resolveConfigWorkflow()
	workflow := resolvedConfig(&wf, override)["workflow"].(map[string]any)

	field := func(name string) map[string]any { return workflow[name].(map[string]any) }
	assert.Equal(t, "override", field("review_clean_rounds")["source"], "task-file value reports override")
	assert.Equal(t, 3, field("review_clean_rounds")["value"])
	assert.Equal(t, "project", field("review_max_rounds")["source"], "project value reports project")
	assert.Equal(t, "default", field("audit_clean_rounds")["source"], "unset value reports default")
}

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestZeroConfig_BehavesLikeV023 proves that with no .tp/ directory present,
// workflow resolution and task-file discovery behave exactly as in v0.23.0:
// the effective workflow equals the task file's own block and discovery uses
// the legacy chain.
func TestZeroConfig_BehavesLikeV023(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":4,"quality_gate":"make test"}}`), 0o600))
	t.Chdir(root)

	// No .tp/: the effective workflow is exactly the task file's own block.
	wf := EffectiveWorkflowForTaskFile("s.tasks.json")
	assert.Equal(t, 4, wf.ReviewMaxRounds)
	assert.Equal(t, "make test", wf.QualityGate)
	assert.Equal(t, 2, wf.ReviewCleanRounds, "unset fields keep the v0.23.0 built-in default")

	// Discovery works with no .tp/ (auto-detect), unchanged from v0.23.0.
	got, err := DiscoverTaskFile(root, "")
	require.NoError(t, err)
	assert.Contains(t, got, "s.tasks.json")

	// A legacy .tp-active marker still works with no .tp/ present.
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp-active"), []byte("s.tasks.json\n"), 0o600))
	got, err = DiscoverTaskFile(root, "")
	require.NoError(t, err)
	assert.Contains(t, got, "s.tasks.json")
}

// TestOptIn_ExistingFullTaskFileWinsAndIsNotRewritten proves adoption is
// opt-in: an existing task file with a full workflow block keeps overriding the
// project config (nothing is thinned) and reading never rewrites the file.
func TestOptIn_ExistingFullTaskFileWinsAndIsNotRewritten(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":99}}`), 0o600))
	full := `{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":4,"review_clean_rounds":3}}`
	tfPath := filepath.Join(root, "s.tasks.json")
	require.NoError(t, os.WriteFile(tfPath, []byte(full), 0o600))
	t.Chdir(root)

	wf := EffectiveWorkflowForTaskFile("s.tasks.json")
	assert.Equal(t, 4, wf.ReviewMaxRounds, "the existing full override wins over the project config")
	assert.Equal(t, 3, wf.ReviewCleanRounds)

	after, err := os.ReadFile(tfPath)
	require.NoError(t, err)
	assert.JSONEq(t, full, string(after), "resolving never auto-thins an existing task file")
}

// TestNonGoals_NoUserHomeConfig proves resolution is exactly two layers
// (project + per-task): a user-home ~/.config/tp config is never consulted, so
// v0.24.0 introduces no user-global layer.
func TestNonGoals_NoUserHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":77}}`), 0o600))

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	t.Chdir(root)

	// No project config and no task override: only the built-in default applies;
	// the user-home config is ignored.
	wf := EffectiveWorkflowForTaskFile("nonexistent.tasks.json")
	assert.Equal(t, 0, wf.ReviewMaxRounds, "a user-home config is not consulted; the built-in default applies")
	assert.Equal(t, 2, wf.ReviewCleanRounds, "resolution stays two-layer with no user-global source")
}

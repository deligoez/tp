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

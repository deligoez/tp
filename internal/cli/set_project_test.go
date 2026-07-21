package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSetProjectWorkflow_WritesConfigAtProjectRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	t.Chdir(root)

	require.NoError(t, runSetProjectWorkflow([]string{"review_max_rounds=8"}))

	// The field is written to a new .tp/config.json at the git-boundary root.
	pc, _, err := engine.LoadProjectConfig(filepath.Join(root, ".tp"))
	require.NoError(t, err)
	require.NotNil(t, pc.Workflow.ReviewMaxRounds)
	assert.Equal(t, 8, *pc.Workflow.ReviewMaxRounds)

	// A .gitignore keeps local.json out of version control.
	data, err := os.ReadFile(filepath.Join(root, ".tp", ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "local.json")
}

func TestRunSetProjectWorkflow_QualityGateAuthorable(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	t.Chdir(root)

	// quality_gate is read-only per task but authorable at the project level.
	require.NoError(t, runSetProjectWorkflow([]string{"quality_gate=go test ./..."}))
	pc, _, err := engine.LoadProjectConfig(filepath.Join(root, ".tp"))
	require.NoError(t, err)
	require.NotNil(t, pc.Workflow.QualityGate)
	assert.Equal(t, "go test ./...", *pc.Workflow.QualityGate)
}


package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEffectiveWorkflow_OutOfRangeTaskOverrideClamped(t *testing.T) {
	// No project config: an out-of-range task override clamps to the built-in
	// default and produces an "out of range" warning (§7.1/§10.8).
	root := t.TempDir()
	wf, warnings, err := ResolveEffectiveWorkflow(root, model.WorkflowOverride{ReviewCleanRounds: ptr(0)})
	require.NoError(t, err)
	assert.Equal(t, 2, wf.ReviewCleanRounds, "out-of-range clamps to the built-in default")
	assert.True(t, strings.Contains(strings.Join(warnings, " "), "out of range"), "an out-of-range value warns")
}

func TestResolveEffectiveWorkflow_OutOfRangeDoesNotMaskProject(t *testing.T) {
	// An out-of-range task override must not mask a project-config value: it
	// clamps to absent and resolves through the project layer (§3.5/§10.8).
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_clean_rounds":3}}`), 0o600))

	wf, _, err := ResolveEffectiveWorkflow(root, model.WorkflowOverride{ReviewCleanRounds: ptr(0)})
	require.NoError(t, err)
	assert.Equal(t, 3, wf.ReviewCleanRounds, "clamped override resolves through the project layer, not the built-in")
}

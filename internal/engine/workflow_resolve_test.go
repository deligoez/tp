package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

func writeTaskFileJSON(t *testing.T, path, spec string, reviewCleanRounds int) {
	t.Helper()
	tf := &model.TaskFile{
		Version: 1,
		Spec:    spec,
		Workflow: model.WorkflowOverride{
			ReviewCleanRounds:  ptr(reviewCleanRounds),
			AuditCleanRounds:   ptr(2),
			GateTimeoutSeconds: ptr(600),
			Checks:             &[]model.Check{},
		},
		Coverage: model.Coverage{ContextOnly: []string{}, Unmapped: []string{}},
		Tasks:    []model.Task{},
	}
	require.NoError(t, model.WriteTaskFile(path, tf))
}

func TestResolveWorkflow_DiscoveryChainMatch(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec"), 0o600))
	tfPath := filepath.Join(dir, "custom.tasks.json")
	writeTaskFileJSON(t, tfPath, "spec.md", 5)

	wf, src := ResolveWorkflow(specPath, tfPath)
	assert.Equal(t, 5, wf.ReviewCleanRounds)
	assert.Equal(t, tfPath, src)
}

func TestResolveWorkflow_SpecMismatchFallsToAdjacent(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec"), 0o600))

	otherPath := filepath.Join(dir, "other.tasks.json")
	writeTaskFileJSON(t, otherPath, "other.md", 7)

	adjacentPath := filepath.Join(dir, "spec.tasks.json")
	writeTaskFileJSON(t, adjacentPath, "spec.md", 3)

	wf, src := ResolveWorkflow(specPath, otherPath)
	assert.Equal(t, 3, wf.ReviewCleanRounds)
	assert.Equal(t, adjacentPath, src)
}

func TestResolveWorkflow_Defaults(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec"), 0o600))

	wf, src := ResolveWorkflow(specPath, "")
	assert.Equal(t, "", src)
	assert.Equal(t, 2, wf.ReviewCleanRounds)
	assert.Equal(t, 2, wf.AuditCleanRounds)
	assert.Equal(t, 600, wf.GateTimeoutSeconds)
	assert.Equal(t, 5, wf.LockTimeoutSeconds)
	assert.Equal(t, 0, wf.ReviewMaxRounds)
	assert.Equal(t, 0, wf.AuditMaxRounds)
	assert.Equal(t, []model.Check{}, wf.Checks)
}

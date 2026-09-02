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

	// t.Chdir, because ResolveWorkflow discovers the PROJECT layer by walking up
	// from the working directory (workflow_resolve.go:20), not from specPath.
	// Under `go test` that directory is inside this repository, so without this
	// the test resolved tp's own .tp/config.json and asserted against it.
	//
	// It passed anyway for as long as every value the repo set happened to equal
	// the built-in default — review_clean_rounds and audit_clean_rounds are both
	// 2 in each. Registering `checks` at the project layer was the first value to
	// diverge, and the assertion this test had never really made finally failed.
	// A test that reaches the repository is a test whose verdict the repository
	// can change.
	t.Chdir(dir)
	require.NoFileExists(t, filepath.Join(dir, ".tp", "config.json"),
		"the fixture must have no project layer, or this asserts the defaults against a config")

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

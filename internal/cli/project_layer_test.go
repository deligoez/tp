package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below are spec/backlog/project-layer-from-task-file.md: the
// project layer belongs to the task file, and work in the working tree is
// refused when the task file's project root is not the working directory's.

// twoProjects builds two unrelated project trees, each with its own
// .tp/config.json naming a gate that leaves a marker file in its own tree, and
// a task file with one open task in the second. It returns the first (the
// working directory a test runs from), the second, and the second's task file.
func twoProjects(t *testing.T) (here, there, taskFile string) {
	t.Helper()
	here, there = t.TempDir(), t.TempDir()
	for dir, gate := range map[string]string{here: "touch here-gate-ran", there: "touch there-gate-ran"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o750))
		cfg, err := json.Marshal(map[string]any{"workflow": map[string]any{"quality_gate": gate}})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"), cfg, 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(there, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	_, stderr, code := runTP(t, there, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, there, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["# Spec"]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	return here, there, filepath.Join(there, "spec.tasks.json")
}

// TestTheProjectLayerIsResolvedFromTheTaskFile: `tp config --file` naming a
// task file in another tree reports that tree's quality_gate, not the working
// directory's.
func TestTheProjectLayerIsResolvedFromTheTaskFile(t *testing.T) {
	t.Parallel()
	here, _, taskFile := twoProjects(t)
	stdout, stderr, code := runTP(t, here, "config", "--file", taskFile)
	require.Equal(t, 0, code, "config: %s", stderr)
	var out struct {
		Workflow struct {
			QualityGate string `json:"quality_gate"`
		} `json:"workflow"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "%s", stdout)
	assert.Equal(t, "touch there-gate-ran", out.Workflow.QualityGate,
		"the task file's own project supplies the project layer")
}

// TestWorkInAnotherTreeIsRefused: `tp done` runs the gate in the working tree,
// so with a task file whose project root is another directory it is refused
// with a hint naming the directory to run it from, and no gate runs anywhere.
func TestWorkInAnotherTreeIsRefused(t *testing.T) {
	t.Parallel()
	here, there, taskFile := twoProjects(t)
	_, stderr, code := runTP(t, here, "done", "--file", taskFile, "t1", "implemented and verified")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
	resolved, err := filepath.EvalSymlinks(there)
	require.NoError(t, err)
	assert.Contains(t, stderr, resolved, "the hint names the directory to run it from")
	assert.NoFileExists(t, filepath.Join(here, "here-gate-ran"), "this tree's gate did not run")
	assert.NoFileExists(t, filepath.Join(there, "there-gate-ran"), "and neither did that tree's, from here")
}

// TestReportsAboutAnotherTreeStayAllowed: what reads only the task file and its
// state is allowed from outside.
func TestReportsAboutAnotherTreeStayAllowed(t *testing.T) {
	t.Parallel()
	here, _, taskFile := twoProjects(t)
	_, stderr, code := runTP(t, here, "status", "--file", taskFile)
	assert.Equal(t, 0, code, "status: %s", stderr)
}

// TestAnUnreadableProjectConfigIsAnError: a .tp/config.json that exists but
// does not parse is an error on the paths that use the project layer, not "no
// project config" — here tp plan, which resolves the workflow it prints.
func TestAnUnreadableProjectConfigIsAnError(t *testing.T) {
	t.Parallel()
	_, there, _ := twoProjects(t)
	require.NoError(t, os.WriteFile(filepath.Join(there, ".tp", "config.json"), []byte("{broken"), 0o600))
	_, stderr, code := runTP(t, there, "plan")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "config.json")
}

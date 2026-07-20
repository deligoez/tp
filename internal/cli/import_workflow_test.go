package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readPersistedWorkflow(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	var tf struct {
		Workflow map[string]any `json:"workflow"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	return tf.Workflow
}

const importWorkflowTask = `{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"setup done","source_sections":["1. Setup"],"depends_on":[]}`

func TestImport_WorkflowPreserved(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		require.NoError(t, os.WriteFile(specPath, []byte(normSpec), 0o600))
		_, stderr, code := runTP(t, dir, "init", "spec.md", "--quality-gate", "echo preserved-gate")
		require.Equal(t, 0, code, "init failed: %s", stderr)
		return dir
	}

	t.Run("bare array force re-import preserves workflow", func(t *testing.T) {
		dir := setup(t)
		_, _, code := runTP(t, dir, "set", "--workflow", `checks=[{"class":"kept-check","cmd":"true"}]`)
		require.Equal(t, 0, code)
		importPath := filepath.Join(dir, "import.json")
		require.NoError(t, os.WriteFile(importPath, []byte(`[`+importWorkflowTask+`]`), 0o600))

		_, stderr, code := runTP(t, dir, "import", importPath, "--spec", "spec.md", "--force")
		require.Equal(t, 0, code, "import failed: %s", stderr)

		wf := readPersistedWorkflow(t, dir)
		assert.Equal(t, "echo preserved-gate", wf["quality_gate"])
		checks := wf["checks"].([]any)
		require.Len(t, checks, 1, "checks preserved through --force re-import")
		assert.Equal(t, "kept-check", checks[0].(map[string]any)["class"])
	})

	t.Run("wrapped import without workflow key preserves workflow", func(t *testing.T) {
		dir := setup(t)
		importPath := filepath.Join(dir, "import.json")
		doc := `{"version":1,"spec":"spec.md","tasks":[` + importWorkflowTask + `]}`
		require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))

		_, stderr, code := runTP(t, dir, "import", importPath, "--force")
		require.Equal(t, 0, code, "import failed: %s", stderr)

		wf := readPersistedWorkflow(t, dir)
		assert.Equal(t, "echo preserved-gate", wf["quality_gate"])
	})

	t.Run("wrapped import with workflow key wins", func(t *testing.T) {
		dir := setup(t)
		importPath := filepath.Join(dir, "import.json")
		doc := `{"version":1,"spec":"spec.md","workflow":{"quality_gate":"echo imported-gate"},"tasks":[` + importWorkflowTask + `]}`
		require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))

		_, stderr, code := runTP(t, dir, "import", importPath, "--force")
		require.Equal(t, 0, code, "import failed: %s", stderr)

		wf := readPersistedWorkflow(t, dir)
		assert.Equal(t, "echo imported-gate", wf["quality_gate"])
	})

	t.Run("new bare array import gets workflow defaults", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		require.NoError(t, os.WriteFile(specPath, []byte(normSpec), 0o600))
		importPath := filepath.Join(dir, "import.json")
		require.NoError(t, os.WriteFile(importPath, []byte(`[`+importWorkflowTask+`]`), 0o600))

		_, stderr, code := runTP(t, dir, "import", importPath, "--spec", "spec.md")
		require.Equal(t, 0, code, "import failed: %s", stderr)

		wf := readPersistedWorkflow(t, dir)
		assert.Equal(t, float64(2), wf["review_clean_rounds"])
		assert.Equal(t, float64(600), wf["gate_timeout_seconds"])
	})
}

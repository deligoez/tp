package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bulletTask's two bullets carry a "; " and a ". " of their own; each is still
// one criterion.
const bulletTask = `{"id":"t1","title":"Bullets","depends_on":[],"estimate_minutes":5,"acceptance":"- first criterion; with a semicolon\n- second. With a period","source_sections":["s1"]}`

// twoEvidenceLines is one column-0 evidence line per bullet of bulletTask.
const twoEvidenceLines = "- first verified at internal/a.go:1\n- second verified at internal/b.go:2"

// criteriaPayload returns the payload's per-task criteria counts, nil when the
// key is absent.
func criteriaPayload(t *testing.T, stdout string) any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "stdout is the payload: %s", stdout)
	return payload["criteria"]
}

func TestBulletAcceptance_EachBulletIsOneCriterion(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	stdout, stderr, code := runTP(t, dir, "add", bulletTask)
	require.Equal(t, 0, code, "add failed: %s", stderr)
	assert.Equal(t, map[string]any{"t1": float64(2)}, criteriaPayload(t, stdout))

	// The atomicity rule must fire on a prose task with four criteria, so its
	// silence on t1 is the count and not a rule that never ran.
	addTask(t, dir, `{"id":"t2","title":"Prose","depends_on":[],"estimate_minutes":5,"acceptance":"A holds. B holds. C holds. D holds.","source_sections":["s1"]}`)
	stdout, stderr, _ = runTP(t, dir, "validate")
	assert.Contains(t, stdout+stderr, "task t2: acceptance has 4 criteria (max 3)")
	assert.NotContains(t, stdout+stderr, "task t1: acceptance has")

	stdout, stderr, code = runTP(t, dir, "done", "t1", "--gate-passed", "--", twoEvidenceLines)
	require.Equal(t, 0, code, "two evidence lines close a two-bullet task: %s", stderr)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "t1", result["closed"])
}

func TestArrayAcceptance_CountsOneCriterionPerElement(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	bulk := filepath.Join(dir, "tasks.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(
		`{"id":"a","title":"Array","depends_on":[],"estimate_minutes":5,"acceptance":["first; with a semicolon","second. With a period"],"source_sections":["s1"]}`+"\n"+
			`{"id":"b","title":"Prose","depends_on":[],"estimate_minutes":5,"acceptance":"Model exists; migration runs. Tests pass.","source_sections":["s1"]}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "add", "--bulk", bulk)
	require.Equal(t, 0, code, "bulk add failed: %s", stderr)
	assert.Equal(t, map[string]any{"a": float64(2), "b": float64(3)}, criteriaPayload(t, stdout))

	_, stderr, code = runTP(t, dir, "done", "a", "--gate-passed", "--", twoEvidenceLines)
	require.Equal(t, 0, code, "two evidence lines close a two-element task: %s", stderr)
}

// TestImport_ReportsCriteriaPerTask also shows the strict import accepting the
// two-bullet task: counted as four criteria, atomicity rejected it.
func TestImport_ReportsCriteriaPerTask(t *testing.T) {
	t.Parallel()
	dir, importPath, _ := importLockSetup(t)

	doc := `{"version":1,"spec":"spec.md","workflow":{},` +
		`"coverage":{"total_sections":0,"mapped_sections":0,"context_only":[],"unmapped":[]},` +
		`"tasks":[{"id":"t1","title":"Bullets","estimate_minutes":5,"depends_on":[],"source_sections":["## 1. Setup"],` +
		`"acceptance":"- first criterion; with a semicolon\n- second. With a period"}]}`
	require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))

	stdout, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s\n%s", stderr, stdout)
	assert.Equal(t, map[string]any{"t1": float64(2)}, criteriaPayload(t, stdout))
}

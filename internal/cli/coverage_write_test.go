package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// coverageSpec has four headings (1 H1 + 3 H2) so AutoFillCoverage has sections to map.
const coverageSpec = `# Coverage App
## 1. Setup
Setup content.
## 2. Models
Model content.
## 3. API
API content.
`

func initCoverageProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(coverageSpec), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	return dir
}

// readCoverage reads the coverage block from the persisted task file.
func readCoverage(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw := readRawTaskFile(t, dir)
	var tf struct {
		Coverage map[string]any `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal(raw, &tf))
	require.NotNil(t, tf.Coverage)
	return tf.Coverage
}

// §7.1: tp init + tp add + tp validate is clean with no import round-trip.
func TestAdd_ComputesCoverage_CleanValidate(t *testing.T) {
	dir := initCoverageProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"setup","title":"Setup","estimate_minutes":5,"acceptance":"setup done","source_sections":["## 1. Setup"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add failed: %s", stderr)

	cov := readCoverage(t, dir)
	// Spec has 4 headings; AutoFillCoverage must have populated them at add time.
	assert.Equal(t, float64(4), cov["total_sections"], "total_sections must reflect spec headings")
	assert.Equal(t, float64(1), cov["mapped_sections"], "the added task maps one heading")

	// tp validate is clean without an import round-trip.
	stdout, stderr, code := runTP(t, dir, "validate")
	require.Equal(t, 0, code, "validate must be clean: stderr=%s stdout=%s", stderr, stdout)
	var valOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &valOut))
	assert.Equal(t, true, valOut["valid"])
	checks, ok := valOut["checks"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pass", checks["coverage"])
}

// §7.1: removing a task that mapped a heading drops the mapped count.
func TestRemove_RecomputesCoverage(t *testing.T) {
	dir := initCoverageProject(t)
	// Two tasks, each mapping a distinct heading → mapped_sections == 2.
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"setup","title":"Setup","estimate_minutes":5,"acceptance":"setup","source_sections":["## 1. Setup"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add setup failed: %s", stderr)
	_, stderr, code = runTP(t, dir, "add",
		`{"id":"models","title":"Models","estimate_minutes":5,"acceptance":"models","source_sections":["## 2. Models"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add models failed: %s", stderr)
	require.Equal(t, float64(2), readCoverage(t, dir)["mapped_sections"])

	_, stderr, code = runTP(t, dir, "remove", "models")
	require.Equal(t, 0, code, "remove failed: %s", stderr)
	assert.Equal(t, float64(1), readCoverage(t, dir)["mapped_sections"], "remove must recompute coverage")
}

// §7.1: setting an anchor field recomputes coverage; a non-anchor field does not.
func TestSet_SourceSections_RecomputesCoverage(t *testing.T) {
	dir := initCoverageProject(t)
	// Anchor via source_lines only → no heading mapped yet.
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"Task","estimate_minutes":5,"acceptance":"done","source_lines":"4-4"}`)
	require.Equal(t, 0, code, "add failed: %s", stderr)
	require.Equal(t, float64(0), readCoverage(t, dir)["mapped_sections"])

	// Setting source_sections recomputes coverage (value is a JSON array).
	_, stderr, code = runTP(t, dir, "set", "t1", `source_sections=["## 1. Setup"]`)
	require.Equal(t, 0, code, "set failed: %s", stderr)
	assert.Equal(t, float64(1), readCoverage(t, dir)["mapped_sections"], "set source_sections must recompute coverage")

	// A non-anchor field must NOT perturb the coverage mapping.
	before := readCoverage(t, dir)
	_, stderr, code = runTP(t, dir, "set", "t1", "estimate_minutes=8")
	require.Equal(t, 0, code, "set estimate failed: %s", stderr)
	after := readCoverage(t, dir)
	assert.Equal(t, before["mapped_sections"], after["mapped_sections"])
	assert.Equal(t, before["context_only"], after["context_only"])
}

// §7.2: when the spec cannot be read, coverage is left untouched and
// tp validate's coverage finding hints the unreadable spec path.
func TestValidate_UnreadableSpec_HintsPathAndLeavesCoverageUntouched(t *testing.T) {
	dir := t.TempDir()
	// init against spec.md but never create the file → spec is unreadable on
	// every write, yet the task file lands at spec.tasks.json (the usual name).
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)

	_, stderr, code = runTP(t, dir, "add",
		`{"id":"t1","title":"Task","estimate_minutes":5,"acceptance":"done","source_lines":"1-1"}`)
	require.Equal(t, 0, code, "add failed: %s", stderr)

	// §7.2: coverage block left untouched.
	assert.Equal(t, float64(0), readCoverage(t, dir)["total_sections"], "coverage must be untouched when spec is unreadable")

	// tp validate surfaces a coverage finding whose hint names the unreadable spec path.
	stdout, _, _ := runTP(t, dir, "validate")
	var valOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &valOut))
	findings, ok := valOut["findings"].([]any)
	require.True(t, ok, "findings present: %v", valOut)
	var covHint string
	for _, f := range findings {
		fm, _ := f.(map[string]any)
		if fm["rule"] == "coverage" {
			if h, ok := fm["hint"].(string); ok {
				covHint = h
			}
		}
	}
	assert.Contains(t, covHint, "spec.md", "coverage finding hint must name the unreadable spec path")
}

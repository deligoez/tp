package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const normSpec = `# Normalize Test
## 1. Setup
Setup section.
## 2. Backend Migration
Migration section.
### 2.1 Schema
Schema section.
`

const ambigSpec = `# Ambig Test
## Setup
Top.
### Setup
Sub.
`

func writeSpecAndImport(t *testing.T, dir, specName, specContent, taskJSON string) (stdout, stderr string, code int) {
	t.Helper()
	specPath := filepath.Join(dir, specName)
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0o600))
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(taskJSON), 0o600))
	return runTP(t, dir, "import", importPath, "--force")
}

func readPersistedSourceSections(t *testing.T, dir, specName string) map[string][]string {
	t.Helper()
	base := strings.TrimSuffix(specName, filepath.Ext(specName))
	tasksPath := filepath.Join(dir, base+".tasks.json")
	data, err := os.ReadFile(tasksPath)
	require.NoError(t, err, "read persisted task file")
	var tf struct {
		Tasks []struct {
			ID             string   `json:"id"`
			SourceSections []string `json:"source_sections"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	out := make(map[string][]string, len(tf.Tasks))
	for _, task := range tf.Tasks {
		out[task.ID] = task.SourceSections
	}
	return out
}

func TestImport_NormalizesPlainTextSourceSections(t *testing.T) {
	dir := t.TempDir()
	taskJSON := `{
		"version": 1,
		"spec": "spec.md",
		"workflow": {},
		"coverage": {"total_sections": 0, "mapped_sections": 0, "context_only": [], "unmapped": []},
		"tasks": [
			{"id":"t1","title":"Plain","estimate_minutes":5,"acceptance":"setup done","source_sections":["1. Setup"],"depends_on":[]}
		]
	}`
	_, stderr, code := writeSpecAndImport(t, dir, "spec.md", normSpec, taskJSON)
	require.Equal(t, 0, code, "import failed: %s", stderr)

	got := readPersistedSourceSections(t, dir, "spec.md")
	assert.Equal(t, []string{"## 1. Setup"}, got["t1"], "plain text should persist as canonical")
}

func TestImport_KeepsCanonicalSourceSections(t *testing.T) {
	dir := t.TempDir()
	taskJSON := `{
		"version": 1,
		"spec": "spec.md",
		"workflow": {},
		"coverage": {"total_sections": 0, "mapped_sections": 0, "context_only": [], "unmapped": []},
		"tasks": [
			{"id":"t1","title":"Canonical","estimate_minutes":5,"acceptance":"migration done","source_sections":["## 2. Backend Migration"],"depends_on":[]}
		]
	}`
	_, stderr, code := writeSpecAndImport(t, dir, "spec.md", normSpec, taskJSON)
	require.Equal(t, 0, code, "import failed: %s", stderr)

	got := readPersistedSourceSections(t, dir, "spec.md")
	assert.Equal(t, []string{"## 2. Backend Migration"}, got["t1"], "canonical input should round-trip")
}

func TestImport_NormalizesMixedFormat(t *testing.T) {
	dir := t.TempDir()
	taskJSON := `{
		"version": 1,
		"spec": "spec.md",
		"workflow": {},
		"coverage": {"total_sections": 0, "mapped_sections": 0, "context_only": [], "unmapped": []},
		"tasks": [
			{"id":"t1","title":"Plain","estimate_minutes":5,"acceptance":"setup done","source_sections":["1. Setup"],"depends_on":[]},
			{"id":"t2","title":"Canonical","estimate_minutes":5,"acceptance":"migration done","source_sections":["## 2. Backend Migration"],"depends_on":[]},
			{"id":"t3","title":"Sub","estimate_minutes":5,"acceptance":"schema done","source_sections":["2.1 Schema"],"depends_on":[]}
		]
	}`
	_, stderr, code := writeSpecAndImport(t, dir, "spec.md", normSpec, taskJSON)
	require.Equal(t, 0, code, "import failed: %s", stderr)

	got := readPersistedSourceSections(t, dir, "spec.md")
	assert.Equal(t, []string{"## 1. Setup"}, got["t1"])
	assert.Equal(t, []string{"## 2. Backend Migration"}, got["t2"])
	assert.Equal(t, []string{"### 2.1 Schema"}, got["t3"])
}

func TestImport_AmbiguousAborts(t *testing.T) {
	dir := t.TempDir()
	taskJSON := `{
		"version": 1,
		"spec": "spec.md",
		"workflow": {},
		"coverage": {"total_sections": 0, "mapped_sections": 0, "context_only": [], "unmapped": []},
		"tasks": [
			{"id":"a1","title":"Ambig","estimate_minutes":5,"acceptance":"done","source_sections":["Setup"],"depends_on":[]}
		]
	}`
	stdout, stderr, code := writeSpecAndImport(t, dir, "spec.md", ambigSpec, taskJSON)
	require.NotEqual(t, 0, code, "expected non-zero exit for ambiguous import")
	combined := stdout + stderr
	assert.Contains(t, combined, "ambiguous", "error should mention ambiguity")
	assert.Contains(t, combined, "## Setup", "error should list ## Setup candidate")
	assert.Contains(t, combined, "### Setup", "error should list ### Setup candidate")
}

func TestImport_UnresolvableTriggersDidYouMean(t *testing.T) {
	dir := t.TempDir()
	taskJSON := `{
		"version": 1,
		"spec": "spec.md",
		"workflow": {},
		"coverage": {"total_sections": 0, "mapped_sections": 0, "context_only": [], "unmapped": []},
		"tasks": [
			{"id":"t1","title":"Typo","estimate_minutes":5,"acceptance":"done","source_sections":["1. Setp"],"depends_on":[]}
		]
	}`
	stdout, stderr, code := writeSpecAndImport(t, dir, "spec.md", normSpec, taskJSON)
	require.NotEqual(t, 0, code, "expected validation failure for unresolvable entry")
	combined := stdout + stderr
	assert.Contains(t, combined, "Expected canonical format", "error should name canonical format")
	assert.Contains(t, combined, "Did you mean", "error should suggest")
	assert.Contains(t, combined, "## 1. Setup", "suggestion should include closest heading")
}

func TestAdd_NormalizesPlainTextSourceSections(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(normSpec), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)

	taskJSON := `{"id":"a1","title":"Plain","estimate_minutes":5,"acceptance":"setup done","source_sections":["1. Setup"],"depends_on":[]}`
	_, stderr, code = runTP(t, dir, "add", taskJSON)
	require.Equal(t, 0, code, "add failed: %s", stderr)

	got := readPersistedSourceSections(t, dir, "spec.md")
	assert.Equal(t, []string{"## 1. Setup"}, got["a1"], "tp add should normalize like tp import")
}

func TestLint_HeadingFieldUnchanged(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	specContent := "# Top\nIntro paragraph.\n## 4. Backend Migration\n1. one\n2. two\n"
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0o600))

	stdout, _, _ := runTP(t, dir, "lint", "spec.md")

	var lint struct {
		StructuredElements struct {
			NumberedLists []struct {
				Heading string `json:"heading"`
			} `json:"numbered_lists"`
		} `json:"structured_elements"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &lint))
	require.NotEmpty(t, lint.StructuredElements.NumberedLists, "expected at least one numbered list")
	assert.Equal(t, "4. Backend Migration", lint.StructuredElements.NumberedLists[0].Heading,
		"lint heading must remain raw text (no ## prefix) for backward compat")
}

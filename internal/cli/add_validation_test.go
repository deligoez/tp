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

// entryValidationSpec is a minimal spec with one resolvable heading so valid
// tasks can be added (source_sections canonicalizes to "## 1. Setup").
const entryValidationSpec = `# Entry Validation
## 1. Setup
Setup content.
## 2. Models
Model content.
`

// initEntryProject creates a temp dir with a spec and an initialized task file.
func initEntryProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(entryValidationSpec), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	return dir
}

// validEntryTask is a task that passes every §6.1 rule.
const validEntryTask = `{"id":"t1","title":"Setup","estimate_minutes":5,"acceptance":"setup done","source_sections":["## 1. Setup"],"depends_on":[]}`

// errJSON parses the structured error object tp writes to stderr in JSON mode.
func errJSON(t *testing.T, stderr string) map[string]any {
	t.Helper()
	trimmed := strings.TrimSpace(stderr)
	require.NotEmpty(t, trimmed, "expected an error on stderr")
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(trimmed), &m), "parse stderr: %q", stderr)
	return m
}

func TestAdd_RejectsBlankID(t *testing.T) {
	dir := initEntryProject(t)
	cases := []string{
		`{"id":"","title":"T","estimate_minutes":5,"acceptance":"done","source_sections":["## 1. Setup"]}`,
		`{"id":"   ","title":"T","estimate_minutes":5,"acceptance":"done","source_sections":["## 1. Setup"]}`,
	}
	for _, taskJSON := range cases {
		_, stderr, code := runTP(t, dir, "add", taskJSON)
		e := errJSON(t, stderr)
		assert.Equal(t, float64(1), e["code"], "blank id must exit 1: %s", taskJSON)
		assert.Contains(t, e["error"], "id", "error must name the id field")
		assert.Contains(t, e["hint"], "id", "hint must name the id field")
		assert.Equal(t, 1, code)
	}
}

func TestAdd_RejectsDuplicateID(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add", validEntryTask)
	require.Equal(t, 0, code, "first add failed: %s", stderr)

	_, stderr, code = runTP(t, dir, "add", validEntryTask)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "duplicate id must exit 1")
	assert.Contains(t, e["error"], "duplicate", "error must name the duplicate")
	assert.Contains(t, e["hint"], "t1", "hint must name the existing task id")
	assert.Equal(t, 1, code)
}

func TestAdd_RejectsBlankTitle(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"","estimate_minutes":5,"acceptance":"done","source_sections":["## 1. Setup"]}`)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "blank title must exit 1")
	assert.Contains(t, e["error"], "title", "error must name the title field")
	assert.Contains(t, e["hint"], "title", "hint must name the title field")
	assert.Equal(t, 1, code)
}

func TestAdd_RejectsBlankAcceptance(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"   ","source_sections":["## 1. Setup"]}`)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "blank acceptance must exit 1")
	assert.Contains(t, e["error"], "acceptance", "error must name the acceptance field")
	assert.Contains(t, e["hint"], "acceptance", "hint must name the acceptance field")
	assert.Equal(t, 1, code)
}

func TestAdd_RejectsMissingSourceAnchor(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"done"}`)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "missing source anchor must exit 1")
	assert.Contains(t, e["error"], "source_sections", "error must mention source_sections")
	assert.Contains(t, e["hint"], "source_sections", "hint must name both anchors")
	assert.Contains(t, e["hint"], "source_lines", "hint must name both anchors")
	assert.Equal(t, 1, code)
}

func TestAdd_SourceLinesSatisfiesAnchor(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"done","source_lines":"2-3"}`)
	require.Equal(t, 0, code, "source_lines alone should satisfy rule 5: %s", stderr)
}

func TestAdd_RejectsUnknownDependency(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"done","source_sections":["## 1. Setup"],"depends_on":["ghost","missing"]}`)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "unknown dependency must exit 1")
	assert.Contains(t, e["error"], "ghost", "error must list unknown id")
	assert.Contains(t, e["error"], "missing", "error must list unknown id")
	assert.Contains(t, e["hint"], "ghost", "hint must list unknown ids")
	assert.Equal(t, 1, code)
}

func TestAdd_RejectsInvalidJSON_UsageWithDecoderInHint(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add", `{not valid json}`)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(2), e["code"], "invalid JSON must exit 2 (usage)")
	assert.Equal(t, "invalid task JSON", e["error"], "decoder detail must NOT be in error")
	hint, ok := e["hint"].(string)
	require.True(t, ok, "hint must be present")
	assert.NotEmpty(t, hint, "decoder detail must go in hint")
	assert.Equal(t, 2, code)
}

func TestAdd_BulkRejectsInvalidJSON_UsageWithDecoderInHint(t *testing.T) {
	dir := initEntryProject(t)
	ndjson := validEntryTask + "\n" + `{bad json}` + "\n"
	bulkPath := filepath.Join(dir, "bulk.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "add", "--bulk", bulkPath)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(2), e["code"], "invalid JSON in bulk must exit 2")
	assert.Contains(t, e["error"], "line 2", "error must name the offending line")
	assert.NotContains(t, e["error"], "invalid character", "decoder detail must not be in error")
	hint, ok := e["hint"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, hint)
	assert.Equal(t, 2, code)
}

func TestAdd_BulkResolvesForwardDependencyInSameBatch(t *testing.T) {
	dir := initEntryProject(t)
	// t1 depends on t2, which appears LATER in the same batch — must succeed
	// because validation runs after the whole batch is staged (§6.1).
	ndjson := `{"id":"t1","title":"First","estimate_minutes":5,"acceptance":"first","source_sections":["## 1. Setup"],"depends_on":["t2"]}` + "\n" +
		`{"id":"t2","title":"Second","estimate_minutes":5,"acceptance":"second","source_sections":["## 2. Models"],"depends_on":[]}` + "\n"
	bulkPath := filepath.Join(dir, "bulk.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "add", "--bulk", bulkPath)
	require.Equal(t, 0, code, "forward in-batch dependency must succeed: %s", stderr)

	// Both tasks are present in the persisted file.
	data := readRawTaskFile(t, dir)
	var tf struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	got := make([]string, 0, len(tf.Tasks))
	for _, tk := range tf.Tasks {
		got = append(got, tk.ID)
	}
	assert.ElementsMatch(t, []string{"t1", "t2"}, got)
}

func TestAdd_BulkRejectsIntraBatchDuplicate(t *testing.T) {
	dir := initEntryProject(t)
	ndjson := validEntryTask + "\n" + validEntryTask + "\n"
	bulkPath := filepath.Join(dir, "bulk.ndjson")
	require.NoError(t, os.WriteFile(bulkPath, []byte(ndjson), 0o600))

	_, stderr, code := runTP(t, dir, "add", "--bulk", bulkPath)
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "intra-batch duplicate must exit 1")
	assert.Contains(t, e["error"], "duplicate")
	assert.Equal(t, 1, code)
}

// readRawTaskFile returns the raw bytes of the persisted task file so tests can
// assert on the literal JSON shape ([] vs null vs omitted).
func readRawTaskFile(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	return data
}

func TestAdd_NormalizesSlicesToEmptyOnWrite(t *testing.T) {
	dir := initEntryProject(t)
	// A task with no tags, no depends_on, and source_lines only (no
	// source_sections) — every task slice is nil before WriteTaskFile runs.
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"done","source_lines":"2-3"}`)
	require.Equal(t, 0, code, "add failed: %s", stderr)

	raw := readRawTaskFile(t, dir)
	s := string(raw)
	// §6.2: every task carries [] (not null, not omitted) for these slices.
	assert.Contains(t, s, `"depends_on": []`, "depends_on must be [] not null")
	assert.Contains(t, s, `"source_sections": []`, "source_sections must be [] not null")
	assert.Contains(t, s, `"tags": []`, "tags must be [] not null/omitted")
	assert.NotContains(t, s, `"depends_on": null`, "no null slices")
	assert.NotContains(t, s, `"tags": null`)
}

func TestImport_NormalizesSlicesToEmptyOnWrite(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(entryValidationSpec), 0o600))
	// Import a task carrying nil slices (no tags/depends_on/source_sections;
	// source_lines satisfies the anchor rule).
	doc := `{"version":1,"spec":"spec.md","workflow":{},"coverage":{"total_sections":0,"mapped_sections":0,"context_only":[],"unmapped":[]},"tasks":[{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"done","source_lines":"2-3"}]}`
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))

	_, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s", stderr)

	s := string(readRawTaskFile(t, dir))
	assert.Contains(t, s, `"depends_on": []`)
	assert.Contains(t, s, `"source_sections": []`)
	assert.Contains(t, s, `"tags": []`)
}

func TestLint_StructuredElementsEmptyAreArrayNotNull(t *testing.T) {
	dir := t.TempDir()
	// A spec with no tables and no numbered lists.
	spec := "# Plain\n\nJust a paragraph, no structured elements.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	stdout, _, code := runTP(t, dir, "lint", "spec.md")
	require.Equal(t, 0, code)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
	se, ok := raw["structured_elements"]
	require.True(t, ok, "structured_elements present")
	// tables and numbered_lists must marshal to "[]" not "null".
	assert.JSONEq(t, `[]`, string(extractSlice(t, se, "tables")), "tables must be [] when empty")
	assert.JSONEq(t, `[]`, string(extractSlice(t, se, "numbered_lists")), "numbered_lists must be [] when empty")
}

func extractSlice(t *testing.T, structuredElementsJSON json.RawMessage, key string) json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(structuredElementsJSON, &m))
	v, ok := m[key]
	require.True(t, ok, "%s present in structured_elements", key)
	return v
}

func TestStats_TagsEmptyIsArrayNotNull(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add", validEntryTask)
	require.Equal(t, 0, code, "add failed: %s", stderr)

	stdout, _, code := runTP(t, dir, "stats")
	require.Equal(t, 0, code)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
	// tags must be "[]" not "null".
	assert.JSONEq(t, `[]`, string(raw["tags"]), "stats.tags must be [] when empty")
}

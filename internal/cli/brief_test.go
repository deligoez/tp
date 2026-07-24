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

// writeTaskFile writes a bare spec.tasks.json directly into dir, for tests that
// need done tasks or commit_files that tp add cannot produce.
func writeTaskFile(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(body), 0o600))
}

func parseJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &m), "stdout was: %s", stdout)
	return m
}

// topKeys returns the top-level object keys of a JSON object in encoding order.
// Go marshals struct fields in declaration order, so this asserts §4.4 key order.
func topKeys(t *testing.T, stdout string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	tok, err := dec.Token()
	require.NoError(t, err)
	delim, ok := tok.(json.Delim)
	require.True(t, ok && delim == '{', "expected JSON object")
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		require.NoError(t, err)
		keys = append(keys, k.(string))
		require.NoError(t, skipValue(dec))
	}
	return keys
}

// skipValue consumes one JSON value (scalar, object, or array) from dec.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); ok && (d == '{' || d == '[') {
		for dec.More() {
			if d == '{' {
				if _, err := dec.Token(); err != nil { // object key
					return err
				}
			}
			if err := skipValue(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil { // closing delim
			return err
		}
	}
	return nil
}

func TestBrief_ExplicitID_JSONPartsAndVerbatimAcceptance(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# S\n\n## 1. A\nDo a thing.\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"a","title":"Task A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"VERBATIM-ACCEPTANCE","source_sections":["## 1. A"]}`)

	stdout, _, code := runTP(t, dir, "brief", "a")
	require.Equal(t, 0, code)

	// §4.4: the five machine parts in fixed key order.
	assert.Equal(t, []string{"identity", "task", "prior_work", "close", "scope"}, topKeys(t, stdout))

	b := parseJSON(t, stdout)
	// §4.3: acceptance is verbatim from the task file.
	task := b["task"].(map[string]any)
	assert.Equal(t, "a", task["id"])
	assert.Equal(t, "VERBATIM-ACCEPTANCE", task["acceptance"])
	// §7: scope carries the fence prohibitions.
	assert.Contains(t, b["scope"].(string), "refactor")
	// §8: close carries the recipe.
	assert.Contains(t, b["close"].(string), "tp done")
	// §4.2: identity is the one-line reset discipline.
	assert.Contains(t, b["identity"].(string), "executes one unit and stops")
}

func TestBrief_NoArg_TargetsSameTaskAsNextPeek(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	// Two ready tasks; tp next orders by estimate asc when dependent-count ties,
	// so the lower-estimate t2 is next.
	runTP(t, dir, "add", `{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":10,"acceptance":"One.","source_sections":["s"]}`)
	runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"Two.","source_sections":["s"]}`)

	peekOut, _, code := runTP(t, dir, "next", "--peek")
	require.Equal(t, 0, code)
	want := parseJSON(t, peekOut)["task"].(map[string]any)["id"]

	briefOut, _, code := runTP(t, dir, "brief")
	require.Equal(t, 0, code)
	got := parseJSON(t, briefOut)["task"].(map[string]any)["id"]

	// §9.1: brief targets the next ready task by the same ordering tp next uses.
	assert.Equal(t, want, got)
}

func TestBrief_NoArg_TargetsWIPAndIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"wip","title":"WIP","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["s"]}`)
	runTP(t, dir, "add", `{"id":"ready","title":"Ready","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"Two.","source_sections":["s"]}`)
	runTP(t, dir, "claim", "wip") // -> wip

	// §9.1: with no arg, brief targets the in-progress task.
	out, _, code := runTP(t, dir, "brief")
	require.Equal(t, 0, code)
	assert.Equal(t, "wip", parseJSON(t, out)["task"].(map[string]any)["id"])

	// Read-only: a ready task is not claimed by brief.
	out2, _, code := runTP(t, dir, "brief", "ready")
	require.Equal(t, 0, code)
	assert.Equal(t, "ready", parseJSON(t, out2)["task"].(map[string]any)["id"])
	showOut, _, _ := runTP(t, dir, "show", "ready")
	assert.Equal(t, "open", parseJSON(t, showOut)["status"], "brief must not claim a ready task")
}

func TestBrief_AllDone_Exit4DoneShape(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":{},"tasks":[
		{"id":"a","title":"A","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["s"],"closed_reason":"- done"}
	]}`)

	out, _, code := runTP(t, dir, "brief")
	// §9.1: no task available exits 4 with the {done, message} shape tp next uses.
	assert.Equal(t, 4, code)
	m := parseJSON(t, out)
	assert.Equal(t, true, m["done"])
	assert.Contains(t, m["message"].(string), "All tasks complete")
}

func TestBrief_UnknownID_Exit4DoneShape(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"a","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["s"]}`)

	out, _, code := runTP(t, dir, "brief", "nope")
	// §9.1: an unknown explicit id exits 4 (state) with the {done, message} shape.
	assert.Equal(t, 4, code)
	m := parseJSON(t, out)
	assert.Equal(t, false, m["done"])
	assert.Contains(t, m["message"].(string), "not found")
}

func TestBrief_CompactDropsFileListsAndExcerpt(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# S\n\n## 1. A\nDo a thing.\n## 2. B\nDo b.\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":{},"tasks":[
		{"id":"prior","title":"Prior","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"P one.","source_sections":["## 1. A"],"closed_reason":"- prior done","commit_files":["a.go","b.go"],"commit_shas":["deadbee"]},
		{"id":"unit","title":"Unit","status":"open","depends_on":["prior"],"estimate_minutes":5,"acceptance":"UNIT-ACCEPTANCE","source_sections":["## 2. B"],"source_lines":"5-6"}
	]}`)

	out, _, code := runTP(t, dir, "brief", "--compact")
	require.Equal(t, 0, code)
	b := parseJSON(t, out)

	task := b["task"].(map[string]any)
	// §12.1: spec_excerpt omitted under compact.
	_, hasExcerpt := task["spec_excerpt"]
	assert.False(t, hasExcerpt)
	// …but the acceptance text, close recipe, and scope prohibitions are kept.
	assert.Equal(t, "UNIT-ACCEPTANCE", task["acceptance"])
	assert.Contains(t, b["close"].(string), "tp done")
	assert.Contains(t, b["scope"].(string), "refactor")

	// §12.1: prior-work entries collapse to id and evidence summary (no file lists).
	entries := b["prior_work"].(map[string]any)["entries"].([]any)
	require.Len(t, entries, 1)
	e := entries[0].(map[string]any)
	assert.Equal(t, map[string]any{"id": "prior", "evidence_summary": "- prior done"}, e)
}

func TestBrief_PriorOutOfRange_Exit2(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"a","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["s"]}`)

	// §5.4: --prior outside [0,20] is a usage error (exit 2).
	_, _, code := runTP(t, dir, "brief", "--prior", "99")
	assert.Equal(t, 2, code)
}

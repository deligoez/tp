package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bindingChain is spec/1.5.0.md §10 row 1's fixture: the chain a→b→c→d, with
// c a condition and d done, so one graph carries every refusal.
const bindingChain = `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "w"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "w"}]},
    {"id": "c", "title": "C", "milestone": "A", "condition": true, "acceptance": "x", "depends_on": [{"id": "d", "why": "w"}]},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}}
  ]
}
`

// bindingDir returns a project holding bindingChain as its graph and one task
// file, spec.tasks.json, carrying epic when it is not empty.
func bindingDir(t *testing.T, epic string) string {
	t.Helper()
	dir := epicGraphDir(t, bindingChain)
	body := `{"version":1,"spec":"spec.md","tasks":[]}`
	if epic != "" {
		body = `{"version":1,"spec":"spec.md","epic":"` + epic + `","tasks":[]}`
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(body), 0o600))
	return dir
}

// readBoundEpic returns the epic spec.tasks.json in dir names.
func readBoundEpic(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	var tf struct {
		Epic string `json:"epic"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	return tf.Epic
}

// TestSetEpic_BindsTheTaskFile is §10 row 1's first half: tp set --epic b
// writes b into the task file.
func TestSetEpic_BindsTheTaskFile(t *testing.T) {
	t.Parallel()
	dir := bindingDir(t, "")

	_, stderr, code := runTP(t, dir, "set", "--epic", "b")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "b", readBoundEpic(t, dir))
}

// assertBindingRefused runs tp set --epic id on an unbound file and asserts
// exit 1, stderr naming want, and a task file left byte-identical.
func assertBindingRefused(t *testing.T, id, want string) {
	t.Helper()
	dir := bindingDir(t, "")
	path := filepath.Join(dir, "spec.tasks.json")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, "set", "--epic", id)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, want)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "a refused binding writes nothing")
}

// TestSetEpic_AbsentIDIsRefusedAndWritesNothing is §10 row 1: a binding
// written before the id is checked would bind the file to nothing.
func TestSetEpic_AbsentIDIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	assertBindingRefused(t, "z", "not in the graph")
}

// TestSetEpic_ConditionIsRefusedAndWritesNothing is §10 row 1: a condition
// is decided by the operator, so no task file builds it.
func TestSetEpic_ConditionIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	assertBindingRefused(t, "c", "condition")
}


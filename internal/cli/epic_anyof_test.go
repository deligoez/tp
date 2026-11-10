package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anyOfGraph is track [A] with four root epics w, x, y and z, none
// depending on another: room for any-of groups between them.
const anyOfGraph = `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "w", "title": "W", "milestone": "A", "root": true},
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "y", "title": "Y", "milestone": "A", "root": true},
    {"id": "z", "title": "Z", "milestone": "A", "root": true}
  ]
}
`

// storedDeps returns id's depends_on entries as the file holds them, each
// entry's keys decoded raw, so a test sees which keys a write stored.
func storedDeps(t *testing.T, dir, id string) []map[string]any {
	t.Helper()
	var g struct {
		Epics []struct {
			ID        string           `json:"id"`
			DependsOn []map[string]any `json:"depends_on"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	for _, e := range g.Epics {
		if e.ID == id {
			return e.DependsOn
		}
	}
	t.Fatalf("no epic %q in the file", id)
	return nil
}

// anyOfEntry is the stored form of a group: any_of and why, no id.
func anyOfEntry(why string, members ...any) map[string]any {
	return map[string]any{"any_of": members, "why": why}
}

// TestEpicSet_DependsOnTakesAnAnyOfGroup (spec/1.7.0.md §7): the strict
// decode of depends_on= accepts a group, and the file stores it as
// {"any_of": [...], "why": ...} with no id key.
func TestEpicSet_DependsOnTakesAnAnyOfGroup(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfGraph)
	_, stderr, code := runTP(t, dir, "epic", "set", "w", `depends_on=[{"any_of": ["x", "y"], "why": "either opens the API"}]`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, []map[string]any{anyOfEntry("either opens the API", "x", "y")}, storedDeps(t, dir, "w"))
}

// TestEpicAdd_TakesAnAnyOfGroup: tp epic add decodes a group entry strictly
// and stores it as given.
func TestEpicAdd_TakesAnAnyOfGroup(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfGraph)
	_, stderr, code := runTP(t, dir, "epic", "add",
		`{"id": "v", "title": "V", "milestone": "A", "depends_on": [{"any_of": ["x", "y"], "why": "either opens the API"}]}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, []map[string]any{anyOfEntry("either opens the API", "x", "y")}, storedDeps(t, dir, "v"))
}

// TestEpicAdd_BulkTakesAnAnyOfGroup: a --bulk row may carry a group, its
// members naming rows of the same payload.
func TestEpicAdd_BulkTakesAnAnyOfGroup(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfGraph)
	bulk := filepath.Join(dir, "rows.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(
		`{"id": "v", "title": "V", "milestone": "A", "depends_on": [{"any_of": ["u", "x"], "why": "either opens the API"}]}`+"\n"+
			`{"id": "u", "title": "U", "milestone": "A", "root": true}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "epic", "add", "--bulk", bulk)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, []map[string]any{anyOfEntry("either opens the API", "u", "x")}, storedDeps(t, dir, "v"))
}

// TestEpicImport_TakesAnAnyOfGroup: tp epic import's strict decode accepts a
// group entry and writes it as the payload holds it.
func TestEpicImport_TakesAnAnyOfGroup(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, "")
	payload := epicImportPayload(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "v", "title": "V", "milestone": "A", "depends_on": [{"any_of": ["x", "y"], "why": "either opens the API"}]},
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "y", "title": "Y", "milestone": "A", "root": true}
  ]
}
`)
	_, stderr, code := runTP(t, dir, "epic", "import", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, []map[string]any{anyOfEntry("either opens the API", "x", "y")}, storedDeps(t, dir, "v"))
}


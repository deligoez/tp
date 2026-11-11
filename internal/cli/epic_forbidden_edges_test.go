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

// forbiddenEdgesGraph is spec/1.8.0.md §10's rule over a two-track graph: the
// track holding B1 may not depend on milestone M3.
const forbiddenEdgesGraph = `{
  "version": 1,
  "milestones": [
    [
      "M1",
      "M3"
    ],
    [
      "B1"
    ]
  ],
  "forbidden_edges": [
    {
      "from": {
        "track_of": "B1"
      },
      "to": {
        "milestone": "M3"
      },
      "why": "track B ships before M3"
    }
  ],
  "epics": [
    {
      "id": "m",
      "title": "M",
      "milestone": "M1",
      "root": true
    },
    {
      "id": "b",
      "title": "B",
      "milestone": "B1",
      "root": true
    }
  ]
}
`

// forbiddenRule decodes the graph file's forbidden_edges raw.
func forbiddenRules(t *testing.T, dir string) json.RawMessage {
	t.Helper()
	var g map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	return g["forbidden_edges"]
}

// TestEpicGraph_ForbiddenEdgesAreAcceptedAndCarriedByEveryWriter is §10's
// schema: a graph holding forbidden_edges lints clean of unknown-key, keeps
// version 1, and every writer carries the rule through: a set, an add, a
// link and an import. A model without the key drops it on the first write.
func TestEpicGraph_ForbiddenEdgesAreAcceptedAndCarriedByEveryWriter(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, forbiddenEdgesGraph)
	want := forbiddenRules(t, dir)

	out, stderr, code := runEpicLint(t, dir)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	for _, f := range out.Findings {
		assert.NotEqual(t, "unknown-key", f.Rule, "%+v", f)
	}

	writes := [][]string{
		{"epic", "set", "m", "title=Main"},
		{"epic", "add", `{"id": "c", "title": "C", "milestone": "M3", "depends_on": [{"id": "m", "why": "c extends the main line"}]}`},
		{"epic", "link", "c", "m", "c builds on the main line"},
	}
	for _, args := range writes {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "%v: %s", args, stderr)
		assert.JSONEq(t, string(want), string(forbiddenRules(t, dir)), "%v carries forbidden_edges", args)
	}

	importPath := filepath.Join(dir, "graph.json")
	require.NoError(t, os.WriteFile(importPath, []byte(forbiddenEdgesGraph), 0o600))
	_, stderr, code = runTP(t, dir, "epic", "import", importPath, "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, forbiddenEdgesGraph, string(readEpicGraphFile(t, dir)),
		"an imported graph round-trips byte-identically, version 1 included")
}

// TestEpicGraph_AMalformedSelectorIsASchemaError is §10's selector shape:
// a selector with no key, two keys, or a key outside epic, milestone, tag and
// track_of is refused by tp epic import (exit 2) and fails every reader of a
// hand-edited file as a type error does (exit 3). A selector decoded as a
// plain struct accepts all three.
func TestEpicGraph_AMalformedSelectorIsASchemaError(t *testing.T) {
	t.Parallel()
	for name, sel := range map[string]string{
		"no key":      `{}`,
		"two keys":    `{"milestone": "M3", "tag": "ui"}`,
		"unknown key": `{"track": "B1"}`,
	} {
		bad := strings.Replace(forbiddenEdgesGraph, "{\n        \"milestone\": \"M3\"\n      }", sel, 1)
		require.NotEqual(t, forbiddenEdgesGraph, bad, "the fixture holds the selector to replace")

		dir := epicGraphDir(t, "")
		importPath := filepath.Join(dir, "graph.json")
		require.NoError(t, os.WriteFile(importPath, []byte(bad), 0o600))
		_, stderr, code := runTP(t, dir, "epic", "import", importPath)
		assert.Equal(t, 2, code, "%s: import exits 2: %s", name, stderr)
		_, err := os.Stat(filepath.Join(dir, ".tp", "epics.json"))
		assert.True(t, os.IsNotExist(err), "%s: a refused import writes nothing", name)

		hand := epicGraphDir(t, bad)
		_, stderr, code = runTP(t, hand, "epic", "lint")
		assert.Equal(t, 3, code, "%s: a hand-edited file fails the parse: %s", name, stderr)
	}
}

// TestEpicGraph_AnUnknownKeyInAForbiddenRuleIsAnUnknownKey: a hand-edited
// rule carrying a key outside from, to and why lints as unknown-key at its
// path, as an unknown key anywhere else in the graph does.
func TestEpicGraph_AnUnknownKeyInAForbiddenRuleIsAnUnknownKey(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(forbiddenEdgesGraph, `"why": "track B ships before M3"`, `"why": "track B ships before M3", "owner": "x"`, 1)
	require.NotEqual(t, forbiddenEdgesGraph, bad)
	out, stderr, code := runEpicLint(t, epicGraphDir(t, bad))
	require.Equal(t, 1, code, "stderr: %s", stderr)
	var messages []string
	for _, f := range out.Findings {
		if f.Rule == "unknown-key" {
			messages = append(messages, f.Message)
		}
	}
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "forbidden_edges[0].owner")
}

// TestEpicGraph_RemovingAnEpicARuleNamesIsRefused: a write that leaves a
// forbidden_edges selector naming no epic adds a dangling-reference error, so
// tp epic remove of the one epic a rule names exits 1, names the rule's
// selector with the forbidden_edges hint, and writes nothing.
func TestEpicGraph_RemovingAnEpicARuleNamesIsRefused(t *testing.T) {
	t.Parallel()
	graph := strings.Replace(forbiddenEdgesGraph, "\"milestone\": \"M3\"\n      }", "\"epic\": \"m\"\n      }", 1)
	require.NotEqual(t, forbiddenEdgesGraph, graph, "the fixture holds the selector to replace")
	dir := epicGraphDir(t, graph)
	_, stderr, code := runTP(t, dir, "epic", "remove", "m")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `forbidden_edges[0].to epic \"m\" names no epic`)
	assert.Contains(t, stderr, "give each forbidden_edges rule a why")
	assert.Equal(t, graph, string(readEpicGraphFile(t, dir)), "a refused remove writes nothing")
}

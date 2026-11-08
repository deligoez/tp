package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicChainWithGate is §9's chain a → b → c → d in milestone A of tracks
// [[A, B], [L1]], every why distinct, with b in gates.A.members.
const epicChainWithGate = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "gates": {"A": {"members": ["a", "b"]}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "a needs b"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "c needs d"}]},
    {"id": "d", "title": "D", "milestone": "A"}
  ]
}
`

// TestEpicRemove_DependedOnEpicNeedsForceWhichDropsItsEdgesAndGates is §9 row
// 21: a remove that dropped the epic but kept its gate membership would leave
// a dangling-reference, so the file would not lint clean.
func TestEpicRemove_DependedOnEpicNeedsForceWhichDropsItsEdgesAndGates(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "remove", "b")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "a", "the refusal names the dependent epic")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused remove writes nothing")

	_, stderr, code = runTP(t, dir, "epic", "remove", "b", "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var g struct {
		Gates map[string]struct {
			Members []string `json:"members"`
		} `json:"gates"`
		Epics []struct {
			ID        string `json:"id"`
			DependsOn []struct {
				ID string `json:"id"`
			} `json:"depends_on"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	ids := []string{}
	for _, e := range g.Epics {
		ids = append(ids, e.ID)
		if e.ID == "a" {
			assert.Empty(t, e.DependsOn, "the edge a → b is gone")
		}
	}
	assert.Equal(t, []string{"a", "c", "d"}, ids)
	assert.Equal(t, []string{"a"}, g.Gates["A"].Members, "b's gate membership is gone")

	out, stderr, code := runEpicLint(t, dir)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Zero(t, out.Errors, "the file lints clean after the forced remove")
}

// TestEpicRemove_ForceRefusesATagCheckProvider is D8: --force does not remove
// an epic a tag_checks entry names as provider, and writes nothing.
func TestEpicRemove_ForceRefusesATagCheckProvider(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "tag_checks": {"ui": {"provided_by": "p"}},
  "epics": [
    {"id": "p", "title": "P", "milestone": "A"}
  ]
}
`)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "remove", "p", "--force")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "ui", "the refusal names the tag check")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused remove writes nothing")
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicSet_MilestonesWithNoFileCreatesIt is the first half of §9 row 20: a
// set that refused an absent file would leave no graph file behind.
func TestEpicSet_MilestonesWithNoFileCreatesIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, "")
	_, err := os.Stat(filepath.Join(dir, ".tp", "epics.json"))
	require.True(t, os.IsNotExist(err), "the fixture holds no graph file")

	stdout, stderr, code := runTP(t, dir, "epic", "set", `milestones=[["A"]]`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		File string `json:"file"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, "epics.json", filepath.Base(out.File))

	var g struct {
		Milestones [][]string `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	assert.Equal(t, [][]string{{"A"}}, g.Milestones)
}

// TestEpicSet_FieldAddingAnErrorIsRefusedAndWritesNothing: a set moving an epic
// to a milestone no track names adds unknown-milestone, so it exits 1 and the
// file stays byte-identical.
func TestEpicSet_FieldAddingAnErrorIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"}
  ]
}
`)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "set", "a", "milestone=Z")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "unknown-milestone")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused set writes nothing")
}

// TestEpicSet_GatesAndTagChecksReplaceTheWholeObject: a set merging into the
// existing object would keep gate A and tag check det.
func TestEpicSet_GatesAndTagChecksReplaceTheWholeObject(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A", "B"]],
  "gates": {"A": {"members": ["a"]}, "B": {"members": ["b"]}},
  "tag_checks": {"det": {"provided_by": "b"}, "ui": {"provided_by": "a"}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"},
    {"id": "b", "title": "B", "milestone": "B"}
  ]
}
`)
	_, stderr, code := runTP(t, dir, "epic", "set", `gates={"B": {"members": ["b"]}}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = runTP(t, dir, "epic", "set", `tag_checks={"ui": {"provided_by": "b"}}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var g struct {
		Gates     map[string]json.RawMessage `json:"gates"`
		TagChecks map[string]struct {
			ProvidedBy string `json:"provided_by"`
		} `json:"tag_checks"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	assert.Len(t, g.Gates, 1)
	assert.Contains(t, g.Gates, "B")
	require.Len(t, g.TagChecks, 1)
	assert.Equal(t, "b", g.TagChecks["ui"].ProvidedBy)
}

// TestEpicSet_GatesKeepsAnExistingMetRecord is §8: set gates= replaces the
// object whole, so a writer that did not carry met over would drop A's record.
func TestEpicSet_GatesKeepsAnExistingMetRecord(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "gates": {"A": {"members": ["a"], "threshold": "ships", "met": {"at": "2026-01-02T03:04:05Z", "evidence": "run 7", "threshold": "ships"}}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"}
  ]
}
`)
	_, stderr, code := runTP(t, dir, "epic", "set", `gates={"A": {"members": ["a"], "threshold": "ships twice"}}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var g struct {
		Gates map[string]struct {
			Threshold string `json:"threshold"`
			Met       *struct {
				Evidence string `json:"evidence"`
			} `json:"met"`
		} `json:"gates"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	assert.Equal(t, "ships twice", g.Gates["A"].Threshold, "the payload replaces the gate")
	require.NotNil(t, g.Gates["A"].Met, "the met record is carried over")
	assert.Equal(t, "run 7", g.Gates["A"].Met.Evidence)
}

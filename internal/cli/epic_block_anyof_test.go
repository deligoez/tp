package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groupBlockDir is blockDir with b depending only on one of {c1, c2}, two
// conditions of the open A that only the operator decides.
func groupBlockDir(t *testing.T) string {
	t.Helper()
	dir := blockDir(t, "b", "open")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(`{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"any_of": ["c1", "c2"], "why": "either key opens the API"}]},
    {"id": "c1", "title": "C1", "milestone": "A", "root": true, "condition": true, "acceptance": "keys issued"},
    {"id": "c2", "title": "C2", "milestone": "A", "root": true, "condition": true, "acceptance": "vendor keys issued"}
  ]
}
`), 0o600))
	return dir
}

// groupBlockPayload is what a group-blocked payload carries.
type groupBlockPayload struct {
	Message        string     `json:"message"`
	BlockedByEpics []string   `json:"blocked_by_epics"`
	BlockedByAnyOf [][]string `json:"blocked_by_any_of"`
	Operator       *struct {
		Decision string     `json:"decision"`
		Commands []string   `json:"commands"`
		Choices  [][]string `json:"choices"`
	} `json:"operator"`
}

// TestNext_BlockedByAGroupNamesItAndTheOperatorsChoices is spec/1.7.0.md §7:
// tp next on a file whose epic waits on one of {c1, c2} starts no task,
// says "blocked by one of {c1, c2}", carries blocked_by_any_of beside an
// empty blocked_by_epics, and offers the two records as one list of
// alternatives. A tp next that judged blocked_by_epics alone starts t1.
func TestNext_BlockedByAGroupNamesItAndTheOperatorsChoices(t *testing.T) {
	t.Parallel()
	dir := groupBlockDir(t)
	before := taskFileBytes(t, dir)

	stdout, stderr, code := runTP(t, dir, "next")
	require.Equal(t, 4, code, "stdout: %s stderr: %s", stdout, stderr)
	var p groupBlockPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	assert.Contains(t, p.Message, "blocked by one of {c1, c2}")
	assert.Equal(t, []string{}, p.BlockedByEpics)
	assert.Equal(t, [][]string{{"c1", "c2"}}, p.BlockedByAnyOf)
	require.NotNil(t, p.Operator, "stdout: %s", stdout)
	assert.Equal(t, []string{}, p.Operator.Commands)
	assert.Equal(t, [][]string{{"tp epic done c1 --evidence <evidence>", "tp epic done c2 --evidence <evidence>"}}, p.Operator.Choices)
	assert.Equal(t, before, taskFileBytes(t, dir), "a blocked tp next claims nothing")
}

// TestClaimAndDone_ABlockingGroupIsCarriedInTheirRefusals is spec/1.7.0.md
// §7: tp claim, tp done by id and a tp done --batch row, refused on a file
// whose epic waits on one of {c1, c2}, each exit 1 carrying
// blocked_by_any_of, the field the hint names. A refusal that carried
// blocked_by_epics alone leaves the group out.
func TestClaimAndDone_ABlockingGroupIsCarriedInTheirRefusals(t *testing.T) {
	t.Parallel()
	dir := groupBlockDir(t)
	group := [][]string{{"c1", "c2"}}

	for _, args := range [][]string{{"claim", "t1"}, {"done", "t1", "tested by the fixture"}} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 1, code, "%v stderr: %s", args, stderr)
		var p groupBlockPayload
		require.NoError(t, json.Unmarshal([]byte(stderr), &p), "%v stderr: %s", args, stderr)
		assert.Equal(t, group, p.BlockedByAnyOf, "%v", args)
	}

	batch := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(batch, []byte(`{"id":"t1","reason":"tested by the fixture"}`+"\n"), 0o600))
	stdout, stderr, code := runTP(t, dir, "done", "--batch", batch)
	require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
	var p struct {
		Failures []groupBlockPayload `json:"failures"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	require.Len(t, p.Failures, 1, "stdout: %s", stdout)
	assert.Equal(t, group, p.Failures[0].BlockedByAnyOf)
}

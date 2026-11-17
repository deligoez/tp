package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ownerGraph declares ana and ben and gives one work epic both, ben first,
// and a condition ana; d is an unowned condition.
const ownerGraph = `{
  "version": 1,
  "milestones": [["A"]],
  "owners": [{"handle": "ana", "name": "Ana A."}, {"handle": "ben"}],
  "epics": [
    {"id": "w", "title": "W", "milestone": "A", "root": true, "owner": ["ben", "ana"]},
    {"id": "c", "title": "C", "milestone": "A", "root": true, "condition": true, "owner": ["ana"]},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "condition": true}
  ]
}
`

// graphOwners reads the owners set and each epic's owner list from dir's
// graph file.
func graphOwners(t *testing.T, dir string) (set []map[string]string, owner map[string][]string) {
	t.Helper()
	var g struct {
		Owners []map[string]string `json:"owners"`
		Epics  []struct {
			ID    string   `json:"id"`
			Owner []string `json:"owner"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	owner = map[string][]string{}
	for _, e := range g.Epics {
		owner[e.ID] = e.Owner
	}
	return g.Owners, owner
}

func TestEpicOwner_SetOwnersReplacesTheWholeSet(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ownerGraph)

	_, stderr, code := runTP(t, dir, "epic", "set", `owners=[{"handle":"ana"},{"handle":"ben","name":"Ben B."},{"handle":"cem"}]`)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	set, _ := graphOwners(t, dir)
	assert.Equal(t, []map[string]string{{"handle": "ana"}, {"handle": "ben", "name": "Ben B."}, {"handle": "cem"}}, set)
}

// TestEpicOwner_SetOwnerWritesTheListInOrderAndEmptyUnsetsIt: the list keeps
// the order written, and [] leaves no owner key behind.
func TestEpicOwner_SetOwnerWritesTheListInOrderAndEmptyUnsetsIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ownerGraph)

	_, stderr, code := runTP(t, dir, "epic", "set", "d", `owner=["ben","ana"]`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, owner := graphOwners(t, dir)
	assert.Equal(t, []string{"ben", "ana"}, owner["d"])

	_, stderr, code = runTP(t, dir, "epic", "set", "d", "owner=[]")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, owner = graphOwners(t, dir)
	assert.Nil(t, owner["d"])
	assert.NotContains(t, string(readEpicGraphFile(t, dir)), `"owner": []`)
}

// TestEpicOwner_DroppingAHandleAnEpicStillNamesIsRefused: the set's write
// would add unknown-owner on w, so it exits 1 and writes nothing.
func TestEpicOwner_DroppingAHandleAnEpicStillNamesIsRefused(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ownerGraph)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "set", `owners=[{"handle":"ana"}]`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "unknown-owner")
	assert.Contains(t, stderr, `"ben"`)
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused set writes nothing")
}

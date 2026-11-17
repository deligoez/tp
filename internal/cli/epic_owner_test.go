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

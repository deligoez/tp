package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	assert.Contains(t, stderr, `names owner \"ben\"`)
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused set writes nothing")
}

// TestEpicOwner_ARenameIsOneApplyFile: apply lints once after its last row,
// so re-pointing the epics at a handle the set does not declare yet, then
// replacing the set, is one write.
func TestEpicOwner_ARenameIsOneApplyFile(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ownerGraph)
	rows := filepath.Join(t.TempDir(), "rename.ndjson")
	require.NoError(t, os.WriteFile(rows, []byte(
		`{"op":"set","id":"w","field":"owner","value":["bea","ana"]}`+"\n"+
			`{"op":"set","field":"owners","value":[{"handle":"ana","name":"Ana A."},{"handle":"bea"}]}`+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "epic", "apply", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	set, owner := graphOwners(t, dir)
	assert.Equal(t, []map[string]string{{"handle": "ana", "name": "Ana A."}, {"handle": "bea"}}, set)
	assert.Equal(t, []string{"bea", "ana"}, owner["w"])
}

// TestEpicOwner_ExportThenImportIntoAnEmptyProjectCarriesBothKeys: a graph
// file carries its own owners set, so the export's graph imported elsewhere
// brings the set with the epics that name it.
func TestEpicOwner_ExportThenImportIntoAnEmptyProjectCarriesBothKeys(t *testing.T) {
	t.Parallel()
	src := epicGraphDir(t, ownerGraph)
	stdout, stderr, code := runTP(t, src, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var exported struct {
		Graph json.RawMessage `json:"graph"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &exported))
	file := filepath.Join(t.TempDir(), "graph.json")
	require.NoError(t, os.WriteFile(file, exported.Graph, 0o600))

	dst := epicGraphDir(t, "")
	_, stderr, code = runTP(t, dst, "epic", "import", file)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	wantSet, wantOwner := graphOwners(t, src)
	gotSet, gotOwner := graphOwners(t, dst)
	assert.Equal(t, wantSet, gotSet)
	assert.Equal(t, wantOwner, gotOwner)
	assert.Equal(t, []string{"ben", "ana"}, gotOwner["w"])
}

// ownerReady is the part of tp epic ready's payload the owner tests read.
type ownerReady struct {
	Ready []struct {
		ID    string   `json:"id"`
		Owner []string `json:"owner"`
	} `json:"ready"`
	Operator struct {
		Conditions []struct {
			ID    string   `json:"id"`
			Owner []string `json:"owner"`
		} `json:"conditions"`
	} `json:"operator"`
}

func TestEpicOwner_ReadyEntriesCarryTheirOwnerInOrder(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ownerGraph)

	stdout, stderr, code := runTP(t, dir, "epic", "ready")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out ownerReady
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)

	require.Len(t, out.Ready, 1)
	assert.Equal(t, []string{"ben", "ana"}, out.Ready[0].Owner)
	owners := map[string][]string{}
	for _, c := range out.Operator.Conditions {
		owners[c.ID] = c.Owner
	}
	assert.Equal(t, map[string][]string{"c": {"ana"}, "d": nil}, owners)
}

// readyIDs runs tp epic ready with args and returns the ready and condition
// ids it lists.
func readyIDs(t *testing.T, dir string, args ...string) (ready, conditions []string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "ready"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out ownerReady
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	ready, conditions = []string{}, []string{}
	for _, e := range out.Ready {
		ready = append(ready, e.ID)
	}
	for _, c := range out.Operator.Conditions {
		conditions = append(conditions, c.ID)
	}
	return ready, conditions
}

// TestEpicOwner_ReadyFiltersByOwnerAndUnownedAsAUnion: an entry matches
// when its owner names any --owner given; --unowned adds the ones with none.
func TestEpicOwner_ReadyFiltersByOwnerAndUnownedAsAUnion(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ownerGraph)
	cases := []struct {
		args              []string
		ready, conditions []string
	}{
		{[]string{"--owner", "ana"}, []string{"w"}, []string{"c"}},
		{[]string{"--owner", "ben"}, []string{"w"}, []string{}},
		{[]string{"--unowned"}, []string{}, []string{"d"}},
		{[]string{"--owner", "ben", "--unowned"}, []string{"w"}, []string{"d"}},
		{[]string{"--owner", "ben", "--owner", "ana"}, []string{"w"}, []string{"c"}},
	}
	for _, c := range cases {
		ready, conditions := readyIDs(t, dir, c.args...)
		assert.Equal(t, c.ready, ready, "%v", c.args)
		assert.Equal(t, c.conditions, conditions, "%v", c.args)
	}

	_, stderr, code := runTP(t, dir, "epic", "ready", "--owner", "zed")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "zed")
}

// TestEpicOwner_StatusTakesTheSameFilters: "what of mine is blocked" is a
// status question, so status filters its ready, blocked and condition
// entries as ready does.
func TestEpicOwner_StatusTakesTheSameFilters(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "owners": [{"handle": "ana"}, {"handle": "ben"}],
  "epics": [
    {"id": "w", "title": "W", "milestone": "A", "root": true, "owner": ["ben"]},
    {"id": "x", "title": "X", "milestone": "A", "depends_on": [{"id": "w", "why": "x builds on w"}], "owner": ["ana"]},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "condition": true}
  ]
}
`)
	ids := func(args ...string) (ready, blocked, conditions []string) {
		stdout, stderr, code := runTP(t, dir, append([]string{"epic", "status"}, args...)...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var out struct {
			Milestones []struct {
				Ready    []struct{ ID string } `json:"ready"`
				Blocked  []struct{ ID string } `json:"blocked"`
				Operator struct {
					Conditions []struct{ ID string } `json:"conditions"`
				} `json:"operator"`
			} `json:"milestones"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
		require.Len(t, out.Milestones, 1)
		m := out.Milestones[0]
		ready, blocked, conditions = []string{}, []string{}, []string{}
		for _, e := range m.Ready {
			ready = append(ready, e.ID)
		}
		for _, e := range m.Blocked {
			blocked = append(blocked, e.ID)
		}
		for _, e := range m.Operator.Conditions {
			conditions = append(conditions, e.ID)
		}
		return ready, blocked, conditions
	}

	ready, blocked, conditions := ids("--owner", "ana")
	assert.Equal(t, []string{}, ready)
	assert.Equal(t, []string{"x"}, blocked)
	assert.Equal(t, []string{}, conditions)

	ready, blocked, conditions = ids("--unowned")
	assert.Equal(t, []string{}, ready)
	assert.Equal(t, []string{}, blocked)
	assert.Equal(t, []string{"d"}, conditions)

	ready, blocked, _ = ids()
	assert.Equal(t, []string{"w"}, ready)
	assert.Equal(t, []string{"x"}, blocked)
}

// TestEpicOwner_AGraphWithoutOwnersPrintsNoOwnerAnywhere: with no owners
// declared every read is what it was before owners existed, so no output
// names the word, an epic whose owner a hand edit left behind included.
func TestEpicOwner_AGraphWithoutOwnersPrintsNoOwnerAnywhere(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "w", "title": "W", "milestone": "A", "root": true},
    {"id": "x", "title": "X", "milestone": "A", "depends_on": [{"id": "w", "why": "x builds on w"}], "owner": ["ana"]},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "condition": true}
  ]
}
`)
	for _, args := range [][]string{
		{"epic", "ready"}, {"epic", "ready", "--all"}, {"epic", "status"}, {"epic", "export", "--md"},
	} {
		stdout, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "%v stderr: %s", args, stderr)
		assert.NotContains(t, stdout, "owner", "%v", args)
	}
}

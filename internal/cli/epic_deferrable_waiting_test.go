package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deferrableWaitingGraph is spec/1.8.0.md §7's probe: M's gate holds the
// members a (done) and c (undone) and lists d deferrable, and d depends on
// the undone c.
const deferrableWaitingGraph = `{"version": 1, "milestones": [["M"]],
  "gates": {"M": {"members": ["a", "c"], "deferrable": ["d"]}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "M", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}},
    {"id": "c", "title": "C", "milestone": "M", "root": true},
    {"id": "d", "title": "D", "milestone": "M", "depends_on": [{"id": "c", "why": "d reads c's output"}]}
  ]}`

// milestoneLists decodes every milestone's blocked, ready and claimed ids
// and operator condition ids from tp epic status's or export's stdout.
func milestoneLists(t *testing.T, stdout string) []string {
	t.Helper()
	var p struct {
		Milestones []struct {
			Blocked  []struct{ ID string } `json:"blocked"`
			Ready    []struct{ ID string } `json:"ready"`
			Claimed  []struct{ ID string } `json:"claimed"`
			Operator struct {
				Conditions []struct{ ID string } `json:"conditions"`
			} `json:"operator"`
		} `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	require.NotEmpty(t, p.Milestones, "stdout: %s", stdout)
	ids := []string{}
	for _, m := range p.Milestones {
		for _, list := range [][]struct{ ID string }{m.Blocked, m.Ready, m.Claimed, m.Operator.Conditions} {
			for _, e := range list {
				ids = append(ids, e.ID)
			}
		}
	}
	return ids
}

// TestEpicStatusReadyAndExport_ListNoDeferrableMemberAsWaiting is
// spec/1.8.0.md §7: d, deferrable in M's gate and blocked by the undone
// member c, is in no milestone list of tp epic status or tp epic export and
// in no list of tp epic ready. The export's own item for d keeps the graph's
// fact: it is blocked, by c.
func TestEpicStatusReadyAndExport_ListNoDeferrableMemberAsWaiting(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, deferrableWaitingGraph)

	assert.NotContains(t, milestoneLists(t, runEpicStatus(t, dir)), "d", "tp epic status")

	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, milestoneLists(t, stdout), "d", "tp epic export")
	var export struct {
		Epics []struct {
			ID        string   `json:"id"`
			Blocked   bool     `json:"blocked"`
			BlockedBy []string `json:"blocked_by"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &export))
	for _, e := range export.Epics {
		if e.ID == "d" {
			assert.True(t, e.Blocked, "d's own item says it is blocked")
			assert.Equal(t, []string{"c"}, e.BlockedBy)
		}
	}

	stdout, stderr, code = runTP(t, dir, "epic", "ready", "--all")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, stdout, `"d"`, "tp epic ready --all")
}

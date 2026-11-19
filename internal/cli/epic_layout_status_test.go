package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markGraph holds an epic in each mark the map draws: d1 done, r ready on
// d1, b blocked on r, h held by the condition c, k claimed, and w waiting
// in M2, which opens only once M1's gate is green.
const markGraph = `{
  "version": 1,
  "milestones": [["M1", "M2"]],
  "epics": [
    {"id": "c", "title": "C", "milestone": "M1", "condition": true, "acceptance": "decided"},
    {"id": "d1", "title": "D1", "milestone": "M1"},
    {"id": "r", "title": "R", "milestone": "M1", "depends_on": [{"id": "d1", "why": "r needs d1"}]},
    {"id": "b", "title": "B", "milestone": "M1", "depends_on": [{"id": "r", "why": "b needs r"}]},
    {"id": "h", "title": "H", "milestone": "M1", "depends_on": [{"id": "c", "why": "h waits on the decision"}]},
    {"id": "k", "title": "K", "milestone": "M1"},
    {"id": "w", "title": "W", "milestone": "M2", "depends_on": [{"id": "d1", "why": "w needs d1"}]}
  ]
}
`

// pageMarks runs the page's own markState (pure.js) over tp epic export's
// epics, each closed when its milestone's report is not open, as the map
// colours them, and returns each epic's {mark, hold} by id.
func pageMarks(t *testing.T, export string) map[string]struct{ Mark, Hold string } {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil && os.Getenv("CI") != "" {
		t.Fatal("node is not on PATH under CI: the page's markState must run there")
	}
	if err != nil {
		t.Skip("node is not on PATH")
	}
	pure, err := os.ReadFile("../dashboard/page/pure.js")
	require.NoError(t, err)
	prog := string(pure) + `
const P = module.exports;
const x = ` + export + `;
const reports = {};
x.milestones.forEach((m) => { reports[m.milestone] = m; });
const out = {};
x.epics.forEach((e) => { const r = reports[e.milestone]; const m = P.markState(e, !!r && !r.open); out[e.id] = { Mark: m.mark, Hold: m.hold || "" }; });
process.stdout.write(JSON.stringify(out));
`
	got, err := exec.CommandContext(t.Context(), node, "-e", prog).CombinedOutput()
	require.NoError(t, err, string(got))
	var marks map[string]struct{ Mark, Hold string }
	require.NoError(t, json.Unmarshal(got, &marks), string(got))
	return marks
}

// TestEpicLayout_EpicNodesCarryTheMapsStatusAndDownstream: each epic node's
// status and hold are the mark the dashboard's map colours it by, the
// page's markState over the export's derived fields, and its downstream is
// the export's, the counts the drawer shows.
func TestEpicLayout_EpicNodesCarryTheMapsStatusAndDownstream(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, markGraph)
	_, stderr, code := runTP(t, dir, "epic", "done", "d1", "--evidence", "shipped")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "epic", "claim", "k", "--by", "someone")
	require.Equal(t, 0, code, stderr)

	export, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, stderr)
	marks := pageMarks(t, export)
	var exp struct {
		Epics []struct {
			ID         string          `json:"id"`
			Downstream json.RawMessage `json:"downstream"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal([]byte(export), &exp))
	down := map[string]json.RawMessage{}
	for _, e := range exp.Epics {
		down[e.ID] = e.Downstream
	}

	stdout, stderr, code := runTP(t, dir, "epic", "layout")
	require.Equal(t, 0, code, stderr)
	var plan struct {
		Nodes []struct {
			ID         string          `json:"id"`
			Status     string          `json:"status"`
			Hold       string          `json:"hold"`
			Downstream json.RawMessage `json:"downstream"`
		} `json:"nodes"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	seen := map[string]bool{}
	for _, n := range plan.Nodes {
		assert.Equal(t, marks[n.ID].Mark, n.Status, n.ID)
		assert.Equal(t, marks[n.ID].Hold, n.Hold, n.ID)
		if down[n.ID] == nil {
			assert.Nil(t, n.Downstream, n.ID)
		} else {
			assert.JSONEq(t, string(down[n.ID]), string(n.Downstream), n.ID)
		}
		seen[n.Status] = true
	}
	for _, mark := range []string{"done", "ready", "blocked", "held", "claimed", "waiting"} {
		assert.True(t, seen[mark], "the fixture draws a %s epic: %v", mark, marks)
	}
	assert.NotNil(t, down["r"], "r has a dependent, so the drawer shows its downstream")
}

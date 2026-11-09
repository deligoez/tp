package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGatePayload is the JSON tp epic gate prints (spec/1.5.0.md §7).
type epicGatePayload struct {
	Milestone string `json:"milestone"`
	Green     bool   `json:"green"`
	Members   []struct {
		ID     string `json:"id"`
		State  string `json:"state"`
		Status string `json:"status"`
	} `json:"members"`
	Deferrable []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"deferrable"`
	Threshold        string          `json:"threshold"`
	Met              json.RawMessage `json:"met"`
	ThresholdChanged bool            `json:"threshold_changed"`
}

// runEpicGate runs tp epic gate m in dir and decodes the payload it printed.
func runEpicGate(t *testing.T, dir, m string) (out epicGatePayload, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "gate", m)
	if code == 0 || code == 1 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s stderr: %s", stdout, stderr)
	}
	return out, code
}

// TestEpicGate_ReportsMembersGreenOrRedAndDeferrableApart: A's default gate
// holds a (done) and b (not done), never the condition, and lists the
// deferrable epic apart; one red member makes the gate red and exits 1.
func TestEpicGate_ReportsMembersGreenOrRedAndDeferrableApart(t *testing.T) {
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]],
  "gates": {"A": {"members": [], "deferrable": ["later"]}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true, "done": {"at": "2026-01-05T09:00:00Z", "evidence": "shipped"}},
    {"id": "b", "title": "B", "milestone": "A", "root": true},
    {"id": "go", "title": "Go", "milestone": "A", "root": true, "condition": true},
    {"id": "later", "title": "Later", "milestone": "A", "root": true}
  ]}`)

	out, code := runEpicGate(t, dir, "A")

	assert.Equal(t, 1, code, "a red gate exits 1")
	assert.Equal(t, "A", out.Milestone)
	assert.False(t, out.Green)
	require.Len(t, out.Members, 2)
	assert.Equal(t, "a", out.Members[0].ID)
	assert.Equal(t, "green", out.Members[0].State)
	assert.Equal(t, "done", out.Members[0].Status)
	assert.Equal(t, "b", out.Members[1].ID)
	assert.Equal(t, "red", out.Members[1].State)
	assert.Equal(t, "planned", out.Members[1].Status)
	require.Len(t, out.Deferrable, 1)
	assert.Equal(t, "later", out.Deferrable[0].ID)
	assert.Equal(t, "planned", out.Deferrable[0].Status)
}

// TestEpicGate_ExitsZeroOnlyWhenGreen: with every member done, a threshold
// with no met record holds the gate at exit 1; with its met record the gate
// is green at exit 0, and a threshold edited after the record shows as changed
// while the record stands.
func TestEpicGate_ExitsZeroOnlyWhenGreen(t *testing.T) {
	graph := func(gate string) string {
		return `{"version": 1, "milestones": [["A"]], "gates": {"A": ` + gate + `},
  "epics": [{"id": "a", "title": "A", "milestone": "A", "root": true, "done": {"at": "2026-01-05T09:00:00Z", "evidence": "shipped"}}]}`
	}
	met := `"met": {"at": "2026-01-06T09:00:00Z", "evidence": "dashboard", "threshold": "p95 under 200ms"}`

	out, code := runEpicGate(t, epicGraphDir(t, graph(`{"members": [], "threshold": "p95 under 200ms"}`)), "A")
	assert.Equal(t, 1, code, "a threshold not yet met holds the gate")
	assert.False(t, out.Green)
	assert.Equal(t, "p95 under 200ms", out.Threshold)
	assert.Empty(t, out.Met)

	out, code = runEpicGate(t, epicGraphDir(t, graph(`{"members": [], "threshold": "p95 under 200ms", `+met+`}`)), "A")
	assert.Equal(t, 0, code)
	assert.True(t, out.Green)
	assert.NotEmpty(t, out.Met)
	assert.False(t, out.ThresholdChanged)

	out, code = runEpicGate(t, epicGraphDir(t, graph(`{"members": [], "threshold": "p95 under 150ms", `+met+`}`)), "A")
	assert.Equal(t, 0, code, "the met record stands")
	assert.True(t, out.ThresholdChanged, "a threshold edited after its met record shows as changed")
}

// TestEpicGate_AbsentMilestoneExits4: a milestone no track lists has no gate
// to report.
func TestEpicGate_AbsentMilestoneExits4(t *testing.T) {
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [{"id": "a", "title": "A", "milestone": "A", "root": true}]}`)

	_, code := runEpicGate(t, dir, "Z")

	assert.Equal(t, 4, code)
}

// epicGateMetGraph is §10 row 8's fixture: the chain with a gate on A that
// carries threshold and names no members, so a, b, c and d all hold it.
func epicGateMetGraph(threshold, doneThrough string) string {
	done := `, "done": {"at": "2026-01-05T09:00:00Z", "evidence": "shipped"}`
	epic := func(id, dep string) string {
		s := `{"id": "` + id + `", "title": "` + id + `", "milestone": "A"`
		if dep != "" {
			s += `, "depends_on": [{"id": "` + dep + `", "why": "` + id + ` needs ` + dep + `"}]`
		} else {
			s += `, "root": true`
		}
		if id >= doneThrough {
			s += done
		}
		return s + `}`
	}
	return `{"version": 1, "milestones": [["A", "B"]], "gates": {"A": {"members": [], "threshold": "` + threshold + `"}},
  "epics": [` + epic("a", "b") + `, ` + epic("b", "c") + `, ` + epic("c", "d") + `, ` + epic("d", "") + `]}`
}

// TestEpicGateMet_RefusedUntilMembersDoneAndThresholdSet is §10 row 8: --met
// exits 1 and writes nothing while a member is not done and while the
// threshold is empty, and is accepted once both hold; the gate is then green.
func TestEpicGateMet_RefusedUntilMembersDoneAndThresholdSet(t *testing.T) {
	t.Parallel()
	met := []string{"epic", "gate", "A", "--met", "--evidence", "dashboard"}

	notDone := epicGraphDir(t, epicGateMetGraph("p95 under 200ms", "b"))
	before := readEpicGraphFile(t, notDone)
	_, stderr, code := runTP(t, notDone, met...)
	assert.Equal(t, 1, code, "a member not done refuses the met record; stderr: %s", stderr)
	assert.Contains(t, stderr, `members not done: \"a\"`, "the refusal names the member not done, and only it")
	assert.Equal(t, before, readEpicGraphFile(t, notDone), "a refused --met writes nothing")

	noThreshold := epicGraphDir(t, epicGateMetGraph("", "a"))
	before = readEpicGraphFile(t, noThreshold)
	_, stderr, code = runTP(t, noThreshold, met...)
	assert.Equal(t, 1, code, "an empty threshold refuses the met record; stderr: %s", stderr)
	assert.Contains(t, stderr, "threshold")
	assert.Equal(t, before, readEpicGraphFile(t, noThreshold), "a refused --met writes nothing")

	ready := epicGraphDir(t, epicGateMetGraph("p95 under 200ms", "a"))
	_, stderr, code = runTP(t, ready, met...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out, code := runEpicGate(t, ready, "A")
	assert.Equal(t, 0, code)
	assert.True(t, out.Green, "with every member done and the threshold met the gate is green")
}

package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opensAfterStatusGraph is C2.1's track [P, Q, R] with R opening after P and
// one root epic per milestone; p carries a done record when pDone is set.
func opensAfterStatusGraph(pDone bool) string {
	done := ""
	if pDone {
		done = `, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}`
	}
	return `{
  "version": 1,
  "milestones": [["P", "Q", "R"]],
  "opens_after": {"R": ["P"]},
  "epics": [
    {"id": "p", "title": "P", "milestone": "P", "root": true` + done + `},
    {"id": "q", "title": "Q", "milestone": "Q", "root": true},
    {"id": "r", "title": "R", "milestone": "R", "root": true}
  ]
}
`
}

// opensAfterReport is one tp epic status milestone's opening fields.
type opensAfterReport struct {
	Milestone  string   `json:"milestone"`
	Open       bool     `json:"open"`
	OpensAfter []string `json:"opens_after"`
	WaitingOn  []string `json:"waiting_on"`
}

// statusOpensAfter runs tp epic status in dir and returns each milestone's
// opening fields by name.
func statusOpensAfter(t *testing.T, dir string) map[string]opensAfterReport {
	t.Helper()
	stdout := runEpicStatus(t, dir)
	var r struct {
		Milestones []opensAfterReport `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	out := map[string]opensAfterReport{}
	for _, m := range r.Milestones {
		require.NotNil(t, m.OpensAfter, "%s reports opens_after", m.Milestone)
		require.NotNil(t, m.WaitingOn, "%s reports waiting_on", m.Milestone)
		out[m.Milestone] = m
	}
	return out
}

// TestEpicStatus_ReportsOpensAfterAndWaitingOn is C2.6: P reports
// opens_after [], Q [P] by default and R [P] as written; while P is red Q and
// R wait on P, and once P is green both are open with waiting_on [].
func TestEpicStatus_ReportsOpensAfterAndWaitingOn(t *testing.T) {
	t.Parallel()
	ms := statusOpensAfter(t, epicGraphDir(t, opensAfterStatusGraph(false)))
	assert.Equal(t, opensAfterReport{Milestone: "P", Open: true, OpensAfter: []string{}, WaitingOn: []string{}}, ms["P"])
	assert.Equal(t, opensAfterReport{Milestone: "Q", OpensAfter: []string{"P"}, WaitingOn: []string{"P"}}, ms["Q"])
	assert.Equal(t, opensAfterReport{Milestone: "R", OpensAfter: []string{"P"}, WaitingOn: []string{"P"}}, ms["R"])

	ms = statusOpensAfter(t, epicGraphDir(t, opensAfterStatusGraph(true)))
	assert.Equal(t, opensAfterReport{Milestone: "Q", Open: true, OpensAfter: []string{"P"}, WaitingOn: []string{}}, ms["Q"])
	assert.Equal(t, opensAfterReport{Milestone: "R", Open: true, OpensAfter: []string{"P"}, WaitingOn: []string{}}, ms["R"])
}

// TestEpicSet_NullOpensAfterListExits2: a null list is not "always open" nor
// "absent"; set refuses it as a value that is not a list, writing nothing.
func TestEpicSet_NullOpensAfterListExits2(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, opensAfterStatusGraph(false))
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "set", `opens_after={"R":null}`)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `opens_after \"R\"`)
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

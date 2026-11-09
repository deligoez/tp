package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// TestEpicImport_NullOpensAfterListExits2: import refuses the null list set
// refuses, before anything is written.
func TestEpicImport_NullOpensAfterListExits2(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, opensAfterStatusGraph(false))
	before := readEpicGraphFile(t, dir)
	payload := strings.Replace(opensAfterStatusGraph(false), `{"R": ["P"]}`, `{"R": null}`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "g.json"), []byte(payload), 0o600))

	_, stderr, code := runTP(t, dir, "epic", "import", "g.json", "--force")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `opens_after \"R\"`)
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

// lintOpensAfterNotList hand-writes C2.1's graph with R's opens_after value
// replaced by value, runs tp epic lint and returns the opens-after-not-list
// findings, requiring exit 1.
func lintOpensAfterNotList(t *testing.T, value string) []string {
	t.Helper()
	dir := epicGraphDir(t, strings.Replace(opensAfterStatusGraph(false), `{"R": ["P"]}`, `{"R": `+value+`}`, 1))
	stdout, stderr, code := runTP(t, dir, "epic", "lint")
	require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
	var r struct {
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	var messages []string
	for _, f := range r.Findings {
		if f.Rule == "opens-after-not-list" {
			assert.Equal(t, "error", f.Severity)
			messages = append(messages, f.Message)
		}
	}
	return messages
}

// TestEpicLint_NullOpensAfterValueIsAnError: a hand-edited null, which set
// and import refuse, is a lint error naming the milestone.
func TestEpicLint_NullOpensAfterValueIsAnError(t *testing.T) {
	t.Parallel()
	messages := lintOpensAfterNotList(t, "null")
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], `opens_after "R"`)
}

// TestEpicLint_NonListOpensAfterValueIsAnError: a string or an object where a
// list belongs is a lint error naming the milestone, not an unreadable file.
func TestEpicLint_NonListOpensAfterValueIsAnError(t *testing.T) {
	t.Parallel()
	for _, value := range []string{`"P"`, `{"P": true}`, `3`} {
		messages := lintOpensAfterNotList(t, value)
		require.Len(t, messages, 1, "value %s", value)
		assert.Contains(t, messages[0], `opens_after "R"`, "value %s", value)
	}
}

// TestEpicSet_OpensAfterCycleRefusalNamesNoEmptyEpicList: a finding that
// names milestones, not epics, prints no empty "[]" in the refusal.
func TestEpicSet_OpensAfterCycleRefusalNamesNoEmptyEpicList(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, opensAfterStatusGraph(false))

	_, stderr, code := runTP(t, dir, "epic", "set", `opens_after={"P":["Q"]}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "opens-after-cycle (opens_after cycle among milestones P, Q)")
	assert.NotContains(t, stderr, "[]")
}

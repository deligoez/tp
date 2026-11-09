package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicStatus_OpenReportsOpenMilestonesOnly is F7: B waits on A's red
// gate, so --open reports A and C, and the whole graph's critical_path is the
// one the full report carries.
func TestEpicStatus_OpenReportsOpenMilestonesOnly(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"], ["C"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "B", "depends_on": [{"id": "a", "why": "b extends a's model"}]},
    {"id": "c", "title": "C", "milestone": "C", "root": true}
  ]}`)
	var open, full struct {
		CriticalPath json.RawMessage `json:"critical_path"`
		Milestones   []struct {
			Milestone string `json:"milestone"`
		} `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal([]byte(runEpicStatus(t, dir, "--open")), &open))
	require.NoError(t, json.Unmarshal([]byte(runEpicStatus(t, dir)), &full))
	names := []string{}
	for _, m := range open.Milestones {
		names = append(names, m.Milestone)
	}
	assert.Equal(t, []string{"A", "C"}, names)
	assert.Len(t, full.Milestones, 3)
	assert.JSONEq(t, string(full.CriticalPath), string(open.CriticalPath))
}

// TestEpicStatus_OpenOnAClosedMilestoneSaysItIsClosed: --milestone B --open
// asks for an open B; B exists but waits on A, so the call exits 4 saying
// so rather than answering an empty list that reads as "no such milestone".
func TestEpicStatus_OpenOnAClosedMilestoneSaysItIsClosed(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "B", "depends_on": [{"id": "a", "why": "b extends a's model"}]}
  ]}`)
	_, stderr, code := runTP(t, dir, "epic", "status", "--milestone", "B", "--open")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `milestone \"B\" is closed: it waits on A`)
}

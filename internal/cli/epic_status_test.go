package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicPlanReport is tp epic status's payload, reduced to what the tests read.
type epicPlanReport struct {
	Milestones []epicPlanMilestone `json:"milestones"`
}

// epicPlanMilestone is one milestone of tp epic status's payload.
type epicPlanMilestone struct {
	Milestone string         `json:"milestone"`
	Open      bool           `json:"open"`
	Counts    map[string]int `json:"counts"`
	ShareDone float64        `json:"share_done"`
	Gate      struct {
		Green bool `json:"green"`
	} `json:"gate"`
	Ready []struct {
		ID string `json:"id"`
	} `json:"ready"`
	Claimed []struct {
		ID string `json:"id"`
		By string `json:"by"`
	} `json:"claimed"`
	Blocked []struct {
		ID        string   `json:"id"`
		BlockedBy []string `json:"blocked_by"`
	} `json:"blocked"`
	Operator struct {
		Conditions []struct {
			ID string `json:"id"`
		} `json:"conditions"`
	} `json:"operator"`
	CriticalPath struct {
		ByCount []string `json:"by_count"`
	} `json:"critical_path"`
}

// runEpicStatus runs tp epic status with args in dir and returns its stdout.
func runEpicStatus(t *testing.T, dir string, args ...string) string {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "status"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	return stdout
}

// decodeEpicStatus decodes tp epic status's stdout.
func decodeEpicStatus(t *testing.T, stdout string) *epicPlanReport {
	t.Helper()
	var r epicPlanReport
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	return &r
}

// TestEpicStatus_ReportsEachMilestone is spec/1.5.0.md §7: per milestone,
// whether it is open, counts by status and the share done, the gate's state,
// the ready list, the blocked list with each blocking epic and what waits on
// the operator. B is closed while A's gate is red, so w is not ready there.
func TestEpicStatus_ReportsEachMilestone(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}},
    {"id": "z", "title": "Z", "milestone": "A", "root": true},
    {"id": "y", "title": "Y", "milestone": "A", "depends_on": [{"id": "z", "why": "w"}]},
    {"id": "go", "title": "Go", "milestone": "A", "condition": true, "acceptance": "k", "depends_on": [{"id": "x", "why": "w"}]},
    {"id": "w", "title": "W", "milestone": "B", "root": true}
  ]}`)

	r := decodeEpicStatus(t, runEpicStatus(t, dir))

	require.Len(t, r.Milestones, 2)
	a, b := r.Milestones[0], r.Milestones[1]
	assert.Equal(t, "A", a.Milestone)
	assert.True(t, a.Open)
	assert.Equal(t, map[string]int{"planned": 3, "claimed": 0, "specced": 0, "in_progress": 0, "done": 1}, a.Counts)
	assert.InDelta(t, 0.25, a.ShareDone, 1e-9)
	assert.False(t, a.Gate.Green, "z and y are not done")
	require.Len(t, a.Ready, 1)
	assert.Equal(t, "z", a.Ready[0].ID)
	require.Len(t, a.Blocked, 1)
	assert.Equal(t, "y", a.Blocked[0].ID)
	assert.Equal(t, []string{"z"}, a.Blocked[0].BlockedBy, "the blocked list names each blocking epic")
	require.Len(t, a.Operator.Conditions, 1)
	assert.Equal(t, "go", a.Operator.Conditions[0].ID, "a condition with done dependencies waits on the operator")

	assert.Equal(t, "B", b.Milestone)
	assert.False(t, b.Open, "B waits for A's gate")
	assert.Empty(t, b.Ready, "an epic of a closed milestone is not ready")
	assert.Empty(t, b.Blocked)
}

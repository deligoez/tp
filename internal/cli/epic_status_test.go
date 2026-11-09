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
	assert.Equal(t, map[string]int{"planned": 2, "claimed": 0, "specced": 0, "in_progress": 0, "done": 1, "conditions_done": 0}, a.Counts,
		"the condition go is not a work epic (spec/1.6.0.md B2)")
	assert.InDelta(t, 0.33, a.ShareDone, 1e-9)
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

// TestEpicStatus_ConditionsAreNotDeliveredWork pins spec/1.6.0.md B2: a done
// condition counts neither in share_done nor in throughput, the status counts
// cover work epics only, and counts.conditions_done reports it apart. x is
// done in 2026-W01 and the condition go in 2026-W02, so a throughput that
// counted go would list a second week.
func TestEpicStatus_ConditionsAreNotDeliveredWork(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}},
    {"id": "y", "title": "Y", "milestone": "A", "root": true},
    {"id": "go", "title": "Go", "milestone": "A", "condition": true, "acceptance": "k", "depends_on": [{"id": "x", "why": "w"}], "done": {"at": "2026-01-09T03:04:05Z", "evidence": "keys issued"}}
  ]}`)
	var r struct {
		Milestones []struct {
			Counts     map[string]int  `json:"counts"`
			ShareDone  float64         `json:"share_done"`
			Throughput json.RawMessage `json:"throughput"`
		} `json:"milestones"`
	}
	stdout := runEpicStatus(t, dir)
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)

	require.Len(t, r.Milestones, 1)
	m := r.Milestones[0]
	assert.Equal(t, map[string]int{"planned": 1, "claimed": 0, "specced": 0, "in_progress": 0, "done": 1, "conditions_done": 1}, m.Counts)
	assert.InDelta(t, 0.5, m.ShareDone, 1e-9, "x of the work epics x and y is done; go is not work")
	assert.JSONEq(t, `[{"week": "2026-W01", "done": 1}]`, string(m.Throughput), "go's week is not throughput")
}

// TestEpicStatus_CompactLeavesOutTheEstimateChainThroughputAndRollup is
// spec/1.5.0.md §7: under --compact the estimate chain, throughput and the
// roll-up are left out, and since v1.6.0 each milestone's whole critical
// path, which the top-level one carries; without it each is reported.
func TestEpicStatus_CompactLeavesOutTheEstimateChainThroughputAndRollup(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "estimate_minutes": 30, "depends_on": [{"id": "b", "why": "w"}]},
    {"id": "b", "title": "B", "milestone": "A", "root": true, "estimate_minutes": 20}
  ]}`)
	keys := func(stdout string) map[string]json.RawMessage {
		var r struct {
			Milestones []map[string]json.RawMessage `json:"milestones"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
		require.Len(t, r.Milestones, 1)
		return r.Milestones[0]
	}

	full := keys(runEpicStatus(t, dir))
	assert.Contains(t, full, "throughput")
	assert.Contains(t, full, "rollup")
	var fullPath map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(full["critical_path"], &fullPath))
	assert.JSONEq(t, `{"chain": ["b", "a"], "estimate_minutes": 50}`, string(fullPath["by_estimate"]))
	assert.JSONEq(t, `["b", "a"]`, string(fullPath["by_count"]))

	compact := keys(runEpicStatus(t, dir, "--compact"))
	assert.NotContains(t, compact, "throughput")
	assert.NotContains(t, compact, "rollup")
	assert.NotContains(t, compact, "critical_path", "the top-level critical_path carries the chain")
}

// TestEpicStatus_AClaimedEpicShowsItsHolder is C1: tp epic status names the
// holder beside each claimed epic, a blocked one too.
func TestEpicStatus_AClaimedEpicShowsItsHolder(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "a", "why": "w"}], "claimed": {"at": "2026-01-02T03:04:05Z", "by": "agent-1"}}
  ]}`)

	r := decodeEpicStatus(t, runEpicStatus(t, dir))

	require.Len(t, r.Milestones, 1)
	m := r.Milestones[0]
	require.Len(t, m.Claimed, 1)
	assert.Equal(t, "b", m.Claimed[0].ID)
	assert.Equal(t, "agent-1", m.Claimed[0].By, "the claimed epic shows its holder")
	assert.Equal(t, 1, m.Counts["claimed"])
}

// TestEpicStatus_ADoneEpicCutsTheCriticalPath is §10 row 12 with C5: on the
// chain a→b→c→d with b done and A's gate on a, the critical path by count is
// [a], not [a, b, c, d] or [c, d].
func TestEpicStatus_ADoneEpicCutsTheCriticalPath(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "gates": {"A": {"members": ["a"]}}, "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "w"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "w"}], "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "w"}]},
    {"id": "d", "title": "D", "milestone": "A", "root": true}
  ]}`)

	r := decodeEpicStatus(t, runEpicStatus(t, dir))

	require.Len(t, r.Milestones, 1)
	assert.Equal(t, []string{"a"}, r.Milestones[0].CriticalPath.ByCount)
}

// TestEpicStatus_MilestoneNarrowsTheReport is §7: --milestone reports only
// that milestone, and one no track lists exits 4.
func TestEpicStatus_MilestoneNarrowsTheReport(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "y", "title": "Y", "milestone": "B", "root": true}
  ]}`)

	r := decodeEpicStatus(t, runEpicStatus(t, dir, "--milestone", "B"))
	require.Len(t, r.Milestones, 1)
	assert.Equal(t, "B", r.Milestones[0].Milestone)

	_, stderr, code := runTP(t, dir, "epic", "status", "--milestone", "Z")
	assert.Equal(t, 4, code, "stderr: %s", stderr)
}

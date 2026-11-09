package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicReadyReport is tp epic ready's payload, reduced to the ids of each list.
type epicReadyReport struct {
	Ready []struct {
		ID string `json:"id"`
	} `json:"ready"`
	Claimed []struct {
		ID string `json:"id"`
		By string `json:"by"`
	} `json:"claimed"`
	Operator struct {
		Conditions []struct {
			ID string `json:"id"`
		} `json:"conditions"`
		Gates []struct {
			Milestone string `json:"milestone"`
		} `json:"gates"`
	} `json:"operator"`
}

// readyIDs returns the ids of the report's ready list.
func (r *epicReadyReport) readyIDs() []string {
	ids := []string{}
	for _, e := range r.Ready {
		ids = append(ids, e.ID)
	}
	return ids
}

// runEpicReady runs tp epic ready with args in dir and decodes its payload.
func runEpicReady(t *testing.T, dir string, args ...string) *epicReadyReport {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "ready"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var r epicReadyReport
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	return &r
}

// TestEpicReady_ALaterMilestoneWaitsForTheGateUnlessAll is §10 row 6: y in B
// is not ready while A's gate is red with x not done, is listed with --all,
// and is listed once x is done turns A's gate green. A ready list that
// ignores milestone order turns it red.
func TestEpicReady_ALaterMilestoneWaitsForTheGateUnlessAll(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "y", "title": "Y", "milestone": "B", "root": true}
  ]}`)

	assert.Equal(t, []string{"x"}, runEpicReady(t, dir).readyIDs(), "y waits while A's gate is red")
	assert.Equal(t, []string{"x", "y"}, runEpicReady(t, dir, "--all").readyIDs(), "--all lifts the open-milestone rule")

	_, stderr, code := runTP(t, dir, "epic", "done", "x", "--evidence", "shipped")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, []string{"y"}, runEpicReady(t, dir).readyIDs(), "once A's gate is green y is ready")
}

// TestEpicReady_AConditionIsListedApartFromTheWorkEpics is §10 row 10: on the
// chain plus a root condition, the conditions whose dependencies are done, c
// after the done d and the root go, wait on the operator and are not in the
// ready list. A condition listed among the work epics turns it red.
func TestEpicReady_AConditionIsListedApartFromTheWorkEpics(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "w"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "w"}]},
    {"id": "c", "title": "C", "milestone": "A", "condition": true, "acceptance": "x", "depends_on": [{"id": "d", "why": "w"}]},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}},
    {"id": "go", "title": "Go", "milestone": "A", "root": true, "condition": true, "acceptance": "board says yes"}
  ]}`)

	r := runEpicReady(t, dir)
	assert.Empty(t, r.readyIDs(), "no work epic has its dependencies done, and no condition is listed as work")
	conditions := []string{}
	for _, c := range r.Operator.Conditions {
		conditions = append(conditions, c.ID)
	}
	assert.Equal(t, []string{"c", "go"}, conditions, "the conditions with done dependencies wait on the operator")
}

// TestEpicReady_AClaimedEpicIsListedApartWithItsHolder is C1: a claimed epic
// that would otherwise be ready is not in the ready list but in the claimed
// list, naming its holder.
func TestEpicReady_AClaimedEpicIsListedApartWithItsHolder(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "A", "root": true, "claimed": {"at": "2026-01-02T03:04:05Z", "by": "agent-1"}}
  ]}`)

	r := runEpicReady(t, dir)
	assert.Equal(t, []string{"a"}, r.readyIDs(), "a claimed epic is not ready")
	require.Len(t, r.Claimed, 1)
	assert.Equal(t, "b", r.Claimed[0].ID)
	assert.Equal(t, "agent-1", r.Claimed[0].By, "the claimed list names the holder")
}

// TestEpicReady_AGateWhoseThresholdIsUnmetWaitsOnTheOperator is §7: a gate
// whose members are done and whose threshold has no met record waits on the
// operator; once met it is listed no more.
func TestEpicReady_AGateWhoseThresholdIsUnmetWaitsOnTheOperator(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]],
  "gates": {"A": {"members": [], "threshold": "p95 under 200ms"}},
  "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}},
    {"id": "y", "title": "Y", "milestone": "B", "root": true}
  ]}`)

	r := runEpicReady(t, dir)
	require.Len(t, r.Operator.Gates, 1)
	assert.Equal(t, "A", r.Operator.Gates[0].Milestone)
	assert.Empty(t, r.readyIDs(), "B stays closed while A's threshold is unmet")

	_, stderr, code := runTP(t, dir, "epic", "gate", "A", "--met", "--evidence", "run 7")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	r = runEpicReady(t, dir)
	assert.Empty(t, r.Operator.Gates, "a met gate waits on no one")
	assert.Equal(t, []string{"y"}, r.readyIDs())
}

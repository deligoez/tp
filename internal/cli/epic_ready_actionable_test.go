package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// operatorConditions runs tp epic ready with args in dir and returns
// operator.conditions as id -> actionable, failing on a row without the
// actionable key.
func operatorConditions(t *testing.T, dir string, args ...string) map[string]bool {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "ready"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var r struct {
		Operator struct {
			Conditions []struct {
				ID         string `json:"id"`
				Actionable *bool  `json:"actionable"`
			} `json:"conditions"`
		} `json:"operator"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	out := map[string]bool{}
	for _, c := range r.Operator.Conditions {
		require.NotNil(t, c.Actionable, "condition %s carries actionable", c.ID)
		out[c.ID] = *c.Actionable
	}
	return out
}

// conditionWaitGraph is C3.1's graph on track [A]: w depends on the
// condition c and on the work epic u, u done when uDone.
func conditionWaitGraph(uDone bool) string {
	done := ""
	if uDone {
		done = `, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "shipped"}`
	}
	return `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "c", "title": "C", "milestone": "A", "root": true, "condition": true, "acceptance": "the board says yes"},
    {"id": "u", "title": "U", "milestone": "A", "root": true` + done + `},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "c", "why": "w ships once the board agrees"}, {"id": "u", "why": "w extends u's model"}]}
  ]}`
}

// TestEpicReady_C3_1_AConditionBehindUndoneWorkIsNotActionable is C3.1: w
// still waits on the undone work epic u, so deciding c now unblocks nothing.
func TestEpicReady_C3_1_AConditionBehindUndoneWorkIsNotActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, conditionWaitGraph(false))
	assert.Empty(t, operatorConditions(t, dir), "c is not actionable")
	assert.Equal(t, map[string]bool{"c": false}, operatorConditions(t, dir, "--all"))
}

// TestEpicReady_C3_2_OnceTheWorkIsDoneTheConditionIsActionable is C3.2: with
// u done, c is all that holds w.
func TestEpicReady_C3_2_OnceTheWorkIsDoneTheConditionIsActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, conditionWaitGraph(true))
	assert.Equal(t, map[string]bool{"c": true}, operatorConditions(t, dir))
	assert.Equal(t, map[string]bool{"c": true}, operatorConditions(t, dir, "--all"))
}

// TestEpicReady_C3_3_AConditionWithAnUndoneDependencyIsNotActionable is C3.3:
// c itself depends on the undone x.
func TestEpicReady_C3_3_AConditionWithAnUndoneDependencyIsNotActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "c", "title": "C", "milestone": "A", "condition": true, "acceptance": "the board says yes", "depends_on": [{"id": "x", "why": "the board reads x's report"}]},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "c", "why": "w ships once the board agrees"}]}
  ]}`)
	assert.Empty(t, operatorConditions(t, dir))
	assert.Equal(t, map[string]bool{"c": false}, operatorConditions(t, dir, "--all"))
}

// TestEpicReady_C3_4_AConditionInAClosedMilestoneIsNotActionable is C3.4: c
// is in B, which waits on A's red gate.
func TestEpicReady_C3_4_AConditionInAClosedMilestoneIsNotActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "c", "title": "C", "milestone": "B", "root": true, "condition": true, "acceptance": "the board says yes"},
    {"id": "w", "title": "W", "milestone": "B", "depends_on": [{"id": "c", "why": "w ships once the board agrees"}]}
  ]}`)
	assert.Empty(t, operatorConditions(t, dir))
	assert.Equal(t, map[string]bool{"c": false}, operatorConditions(t, dir, "--all"))
}

// TestEpicReady_C3_5_AStandaloneConditionIsActionable is C3.5: nothing
// depends on c and it depends on nothing.
func TestEpicReady_C3_5_AStandaloneConditionIsActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "c", "title": "C", "milestone": "A", "root": true, "condition": true, "acceptance": "the board says yes"}
  ]}`)
	assert.Equal(t, map[string]bool{"c": true}, operatorConditions(t, dir))
}


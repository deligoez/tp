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

// TestEpicReady_C3_6_ConditionsThatAloneHoldAnEpicAreBothActionable is C3.6:
// w waits on c1 and c2 and nothing else.
func TestEpicReady_C3_6_ConditionsThatAloneHoldAnEpicAreBothActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "c1", "title": "C1", "milestone": "A", "root": true, "condition": true, "acceptance": "legal says yes"},
    {"id": "c2", "title": "C2", "milestone": "A", "root": true, "condition": true, "acceptance": "the board says yes"},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "c1", "why": "w ships once legal agrees"}, {"id": "c2", "why": "w ships once the board agrees"}]}
  ]}`)
	assert.Equal(t, map[string]bool{"c1": true, "c2": true}, operatorConditions(t, dir))
}

// TestEpicReady_AConditionHoldingAnOpenGateIsActionable is the deadlock F3
// used to leave: c is an explicit member of M's gate, and its only dependent
// w is in N, which opens after M, with no bound file. Unless c is actionable
// M never goes green, N never opens and nothing moves.
func TestEpicReady_AConditionHoldingAnOpenGateIsActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["M", "N"]],
  "gates": {"M": {"members": ["m", "c"]}},
  "epics": [
    {"id": "m", "title": "M work", "milestone": "M", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "shipped"}},
    {"id": "c", "title": "C", "milestone": "M", "root": true, "condition": true, "acceptance": "the board says yes"},
    {"id": "w", "title": "W", "milestone": "N", "depends_on": [{"id": "c", "why": "w ships once the board agrees"}]}
  ]}`)
	assert.Equal(t, map[string]bool{"c": true}, operatorConditions(t, dir))
}

// readyConditionKeys runs tp epic ready --all in dir and returns each
// operator.conditions row's raw keys by id.
func readyConditionKeys(t *testing.T, dir string) map[string]map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "ready", "--all")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var r struct {
		Operator struct {
			Conditions []map[string]json.RawMessage `json:"conditions"`
		} `json:"operator"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	out := map[string]map[string]json.RawMessage{}
	for _, row := range r.Operator.Conditions {
		var id string
		require.NoError(t, json.Unmarshal(row["id"], &id))
		out[id] = row
	}
	return out
}

// TestEpicReady_NotActionableNamesEveryReasonInOrder: under --all each
// condition that is not actionable carries not_actionable, every reason that
// holds in the order milestone, dependencies, dependents; an actionable one
// carries no such key, and no condition carries waiting_on. c1 is in the
// closed B with its only dependent there unbound; c2 in A waits on the undone
// x and holds w, which waits on it alone; c3 in A holds y, which also waits on
// the undone work epic x; c4 in A stands alone.
func TestEpicReady_NotActionableNamesEveryReasonInOrder(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "c2", "title": "C2", "milestone": "A", "condition": true, "acceptance": "legal says yes", "depends_on": [{"id": "x", "why": "legal reads x's report"}]},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "c2", "why": "w ships once legal agrees"}]},
    {"id": "c3", "title": "C3", "milestone": "A", "root": true, "condition": true, "acceptance": "the board says yes"},
    {"id": "y", "title": "Y", "milestone": "A", "depends_on": [{"id": "c3", "why": "y ships once the board agrees"}, {"id": "x", "why": "y extends x's model"}]},
    {"id": "c4", "title": "C4", "milestone": "A", "root": true, "condition": true, "acceptance": "the auditor signs"},
    {"id": "c1", "title": "C1", "milestone": "B", "condition": true, "acceptance": "the market opens", "depends_on": [{"id": "x", "why": "the market reads x's launch"}]},
    {"id": "z", "title": "Z", "milestone": "B", "depends_on": [{"id": "c1", "why": "z ships once the market opens"}]}
  ]}`)
	rows := readyConditionKeys(t, dir)
	reasons := map[string][]string{}
	for id, row := range rows {
		assert.NotContains(t, row, "waiting_on", "%s carries no waiting_on", id)
		raw, ok := row["not_actionable"]
		if !ok {
			continue
		}
		var list []string
		require.NoError(t, json.Unmarshal(raw, &list))
		reasons[id] = list
	}
	assert.Equal(t, map[string][]string{
		"c1": {"milestone", "dependencies", "dependents"},
		"c2": {"dependencies"},
		"c3": {"dependents"},
	}, reasons)
	assert.Contains(t, rows, "c4", "the actionable c4 is listed without not_actionable")
}

// TestEpicReady_C3_7_ADoneConditionIsListedInNeitherMode is C3.7.
func TestEpicReady_C3_7_ADoneConditionIsListedInNeitherMode(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "c", "title": "C", "milestone": "A", "root": true, "condition": true, "acceptance": "the board says yes", "done": {"at": "2026-01-02T03:04:05Z", "evidence": "minutes"}},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "c", "why": "w ships once the board agrees"}]}
  ]}`)
	assert.Empty(t, operatorConditions(t, dir))
	assert.Empty(t, operatorConditions(t, dir, "--all"))
}

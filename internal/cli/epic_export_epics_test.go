package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportEpicItem is one tp epic export epics[] item as F9 shapes it.
type exportEpicItem struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Status     string           `json:"status"`
	Milestone  string           `json:"milestone"`
	Condition  bool             `json:"condition"`
	Downstream *json.RawMessage `json:"downstream"`
	Actionable *bool            `json:"actionable"`
	HeldBy     []string         `json:"held_by"`
	BlockedBy  []string         `json:"blocked_by"`
}

// exportEpicItems runs tp epic export in dir and returns its epics by id,
// each with the raw keys it carries.
func exportEpicItems(t *testing.T, dir string) (items map[string]exportEpicItem, raw map[string]map[string]json.RawMessage) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var p struct {
		Epics []json.RawMessage `json:"epics"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	items, raw = map[string]exportEpicItem{}, map[string]map[string]json.RawMessage{}
	for _, data := range p.Epics {
		var it exportEpicItem
		require.NoError(t, json.Unmarshal(data, &it))
		var keys map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &keys))
		items[it.ID], raw[it.ID] = it, keys
	}
	return items, raw
}

// exportF9Epics is track [M1, M2]: r is done, the condition k depends on r,
// w depends on k and r, x on w, and y in M2 on x.
func exportF9Epics() []downstreamEpic {
	r := dsEpic("r", "M1")
	r.Root = true
	r.Done = &downstreamDone{At: "2026-01-02T03:04:05Z", Evidence: "e"}
	k := dsEpic("k", "M1", "r")
	k.Condition, k.Acceptance = true, "keys issued"
	w := dsEpic("w", "M1", "k", "r")
	w.Title = "Wire the keys"
	return []downstreamEpic{r, k, w, dsEpic("x", "M1", "w"), dsEpic("y", "M2", "x")}
}

// TestEpicExport_EpicItemsCarryMilestoneConditionDownstreamAndWaits is F9:
// each item repeats title, milestone and condition; w is held by the condition k
// alone, which also blocks it, while r is done and blocks nothing; x is
// blocked by w and held by nothing; k is an actionable condition; the
// downstream counts are F1's without dependents_by_milestone.
func TestEpicExport_EpicItemsCarryMilestoneConditionDownstreamAndWaits(t *testing.T) {
	t.Parallel()
	items, raw := exportEpicItems(t, downstreamDir(t, exportF9Epics(), nil))

	w := items["w"]
	assert.Equal(t, "Wire the keys", w.Title, "the item carries the graph's title")
	assert.Equal(t, "M1", w.Milestone)
	assert.False(t, w.Condition)
	assert.Equal(t, []string{"k"}, w.HeldBy)
	assert.Equal(t, []string{"k"}, w.BlockedBy)
	require.NotNil(t, w.Downstream)
	assert.JSONEq(t, `{"longest_chain": 2, "dependents": 2, "dependents_open": 1}`, string(*w.Downstream))
	assert.Nil(t, w.Actionable, "actionable is a condition's key")

	x := items["x"]
	assert.Equal(t, []string{}, x.HeldBy)
	assert.Equal(t, []string{"w"}, x.BlockedBy)

	k := items["k"]
	assert.True(t, k.Condition)
	require.NotNil(t, k.Actionable)
	assert.True(t, *k.Actionable)
	assert.Equal(t, "M2", items["y"].Milestone)

	r := items["r"]
	assert.Equal(t, "done", r.Status)
	assert.NotContains(t, raw["r"], "downstream", "a done epic has no downstream")
	assert.Equal(t, []string{}, r.HeldBy)
	assert.Equal(t, []string{}, r.BlockedBy)
}

// TestEpicExport_CarriesTheWholeGraphCriticalPath: export holds status's
// top-level critical_path, so one call shows the whole plan.
func TestEpicExport_CarriesTheWholeGraphCriticalPath(t *testing.T) {
	t.Parallel()
	dir := downstreamDir(t, exportF9Epics(), nil)
	var status, export struct {
		CriticalPath *json.RawMessage `json:"critical_path"`
	}
	require.NoError(t, json.Unmarshal([]byte(runEpicStatus(t, dir)), &status))
	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &export))
	require.NotNil(t, export.CriticalPath, "stdout: %s", stdout)
	assert.JSONEq(t, `{"by_count": ["k", "w", "x", "y"]}`, string(*export.CriticalPath))
	assert.JSONEq(t, string(*status.CriticalPath), string(*export.CriticalPath))
}

// TestEpicStatus_BlockedItemsCarryHeldBy: status's blocked items say, as
// export's epics do, which blockers are the operator's conditions.
func TestEpicStatus_BlockedItemsCarryHeldBy(t *testing.T) {
	t.Parallel()
	var status struct {
		Milestones []struct {
			Blocked []struct {
				ID     string   `json:"id"`
				HeldBy []string `json:"held_by"`
			} `json:"blocked"`
		} `json:"milestones"`
	}
	stdout := runEpicStatus(t, downstreamDir(t, exportF9Epics(), nil))
	require.NoError(t, json.Unmarshal([]byte(stdout), &status), "stdout: %s", stdout)
	held := map[string][]string{}
	for _, m := range status.Milestones {
		for _, b := range m.Blocked {
			require.NotNil(t, b.HeldBy, "%s carries held_by", b.ID)
			held[b.ID] = b.HeldBy
		}
	}
	assert.Equal(t, map[string][]string{"w": {"k"}, "x": {}, "y": {}}, held)
}

// conditionBehindWorkGraph is C3.3's track [A]: the condition c depends on the
// undone x, and w depends on c alone.
const conditionBehindWorkGraph = `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "x", "title": "X", "milestone": "A", "root": true},
    {"id": "c", "title": "C", "milestone": "A", "condition": true, "acceptance": "the board says yes", "depends_on": [{"id": "x", "why": "the board reads x's report"}]},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "c", "why": "w ships once the board agrees"}]}
  ]}`

// TestEpicStatus_BlockedConditionsCarryNotActionable: status lists the
// condition c, held by its own undone dependency, among the blocked with
// not_actionable naming why; the work epic w carries no such key, and
// neither carries waiting_on.
func TestEpicStatus_BlockedConditionsCarryNotActionable(t *testing.T) {
	t.Parallel()
	var status struct {
		Milestones []struct {
			Blocked []map[string]json.RawMessage `json:"blocked"`
		} `json:"milestones"`
	}
	stdout := runEpicStatus(t, epicGraphDir(t, conditionBehindWorkGraph))
	require.NoError(t, json.Unmarshal([]byte(stdout), &status), "stdout: %s", stdout)
	rows := map[string]map[string]json.RawMessage{}
	for _, m := range status.Milestones {
		for _, b := range m.Blocked {
			var id string
			require.NoError(t, json.Unmarshal(b["id"], &id))
			rows[id] = b
		}
	}
	require.Contains(t, rows, "c", "stdout: %s", stdout)
	require.Contains(t, rows["c"], "not_actionable", "stdout: %s", stdout)
	assert.JSONEq(t, `["dependencies"]`, string(rows["c"]["not_actionable"]))
	assert.NotContains(t, rows["w"], "not_actionable", "a work epic carries no not_actionable")
	for id, row := range rows {
		assert.NotContains(t, row, "waiting_on", "%s carries no waiting_on", id)
	}
}

// TestEpicExport_ConditionItemsCarryNotActionable: the export's item for the
// condition c, held by its own undone x, says actionable false and why; the
// actionable k of F9's graph and every work epic carry no not_actionable,
// and no item carries waiting_on.
func TestEpicExport_ConditionItemsCarryNotActionable(t *testing.T) {
	t.Parallel()
	_, raw := exportEpicItems(t, epicGraphDir(t, conditionBehindWorkGraph))
	require.Contains(t, raw["c"], "not_actionable")
	assert.JSONEq(t, `["dependencies"]`, string(raw["c"]["not_actionable"]))
	assert.JSONEq(t, `false`, string(raw["c"]["actionable"]))
	for _, id := range []string{"x", "w"} {
		assert.NotContains(t, raw[id], "not_actionable", "the work epic %s carries no not_actionable", id)
	}

	_, raw = exportEpicItems(t, downstreamDir(t, exportF9Epics(), nil))
	assert.JSONEq(t, `true`, string(raw["k"]["actionable"]))
	assert.NotContains(t, raw["k"], "not_actionable", "an actionable condition carries no not_actionable")
	for id, keys := range raw {
		assert.NotContains(t, keys, "waiting_on", "%s carries no waiting_on", id)
	}
}

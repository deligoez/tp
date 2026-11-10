package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportAnyOfGraph is x depending on the work r, on one of the conditions
// {c1, c2} and on one of {w, c1}; y on one of the work {w, r}; z on one of
// {d, m}, satisfied by the done d, so the condition m is moot.
const exportAnyOfGraph = `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "r", "title": "R", "milestone": "A", "root": true},
    {"id": "w", "title": "W", "milestone": "A", "root": true},
    {"id": "c1", "title": "C1", "milestone": "A", "root": true, "condition": true, "acceptance": "keys issued"},
    {"id": "c2", "title": "C2", "milestone": "A", "root": true, "condition": true, "acceptance": "vendor keys issued"},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "m", "title": "M", "milestone": "A", "root": true, "condition": true, "acceptance": "the board says yes"},
    {"id": "x", "title": "X", "milestone": "A", "depends_on": [{"id": "r", "why": "x builds on r"},
      {"any_of": ["c1", "c2"], "why": "either key opens the API"}, {"any_of": ["w", "c1"], "why": "either serves x"}]},
    {"id": "y", "title": "Y", "milestone": "A", "depends_on": [{"any_of": ["w", "r"], "why": "either serves y"}]},
    {"id": "z", "title": "Z", "milestone": "A", "depends_on": [{"any_of": ["d", "m"], "why": "either serves z"}]}
  ]}`

// exportAnyOfWaits is what an export epics[] item says it waits on.
type exportAnyOfWaits struct {
	BlockedBy      []string   `json:"blocked_by"`
	HeldBy         []string   `json:"held_by"`
	BlockedByAnyOf [][]string `json:"blocked_by_any_of"`
	HeldByAnyOf    [][]string `json:"held_by_any_of"`
	Blocked        bool       `json:"blocked"`
	Held           bool       `json:"held"`
}

// TestEpicExport_EpicItemsListEachUnsatisfiedGroupAndSayBlockedAndHeld is
// spec/1.7.0.md §7: blocked_by and held_by stay epic ids, each unsatisfied
// group is one member list in blocked_by_any_of, and in held_by_any_of when
// every member is a condition; blocked and held say whether anything waits,
// so y, which only a group blocks, is blocked, and z, whose group is
// satisfied, is not. An export that read a group's id lists "" in
// blocked_by.
func TestEpicExport_EpicItemsListEachUnsatisfiedGroupAndSayBlockedAndHeld(t *testing.T) {
	t.Parallel()
	_, raw := exportEpicItems(t, epicGraphDir(t, exportAnyOfGraph))
	got := map[string]exportAnyOfWaits{}
	for _, id := range []string{"x", "y", "z"} {
		require.Contains(t, raw, id)
		body, err := json.Marshal(raw[id])
		require.NoError(t, err)
		var w exportAnyOfWaits
		require.NoError(t, json.Unmarshal(body, &w))
		for _, key := range []string{"blocked_by_any_of", "held_by_any_of", "blocked", "held"} {
			assert.Contains(t, raw[id], key, "%s carries %s", id, key)
		}
		got[id] = w
	}
	assert.Equal(t, exportAnyOfWaits{BlockedBy: []string{"r"}, HeldBy: []string{},
		BlockedByAnyOf: [][]string{{"c1", "c2"}, {"w", "c1"}}, HeldByAnyOf: [][]string{{"c1", "c2"}}, Blocked: true, Held: true}, got["x"])
	assert.Equal(t, exportAnyOfWaits{BlockedBy: []string{}, HeldBy: []string{},
		BlockedByAnyOf: [][]string{{"w", "r"}}, HeldByAnyOf: [][]string{}, Blocked: true}, got["y"])
	assert.Equal(t, exportAnyOfWaits{BlockedBy: []string{}, HeldBy: []string{},
		BlockedByAnyOf: [][]string{}, HeldByAnyOf: [][]string{}}, got["z"])
}

// TestEpicExport_AMootEpicCarriesMootInPlaceOfActionable is spec/1.7.0.md
// §7: z's group is satisfied by the done d, so the condition m is moot; its
// item carries moot: true and neither actionable nor not_actionable, and
// an epic that is not moot carries no moot key. An export that judged m
// like any condition gives it actionable: false.
func TestEpicExport_AMootEpicCarriesMootInPlaceOfActionable(t *testing.T) {
	t.Parallel()
	_, raw := exportEpicItems(t, epicGraphDir(t, exportAnyOfGraph))
	require.Contains(t, raw, "m")
	assert.JSONEq(t, `true`, string(raw["m"]["moot"]))
	assert.NotContains(t, raw["m"], "actionable")
	assert.NotContains(t, raw["m"], "not_actionable")
	assert.NotContains(t, raw["c1"], "moot")
	assert.Contains(t, raw["c1"], "actionable")
}

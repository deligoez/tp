package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mootConditionGraph is w depending on one of {c1, c2} with c1 recorded, so
// c2 is moot; c2's own undone dependency x lists it among the blocked.
const mootConditionGraph = `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"any_of": ["c1", "c2"], "why": "either key opens the API"}]},
    {"id": "c1", "title": "C1", "milestone": "A", "root": true, "condition": true, "acceptance": "keys issued",
     "done": {"at": "2026-01-03T03:04:05Z", "evidence": "issued"}},
    {"id": "c2", "title": "C2", "milestone": "A", "condition": true, "acceptance": "vendor keys issued",
     "depends_on": [{"id": "x", "why": "the vendor reads x's contract"}]},
    {"id": "x", "title": "X", "milestone": "A", "root": true}
  ]}`

// TestEpicStatus_AMootConditionCarriesMootInPlaceOfNotActionable is
// spec/1.7.0.md §7: status lists the moot c2, held by its undone x, among
// the blocked with moot: true and no not_actionable, and not among the
// operator's conditions. A status that judged c2 like any condition gives
// it no moot key.
func TestEpicStatus_AMootConditionCarriesMootInPlaceOfNotActionable(t *testing.T) {
	t.Parallel()
	var status struct {
		Milestones []struct {
			Blocked  []map[string]json.RawMessage `json:"blocked"`
			Operator struct {
				Conditions []struct {
					ID string `json:"id"`
				} `json:"conditions"`
			} `json:"operator"`
		} `json:"milestones"`
	}
	stdout := runEpicStatus(t, epicGraphDir(t, mootConditionGraph))
	require.NoError(t, json.Unmarshal([]byte(stdout), &status), "stdout: %s", stdout)
	require.Len(t, status.Milestones, 1, "stdout: %s", stdout)
	rows := map[string]map[string]json.RawMessage{}
	for _, b := range status.Milestones[0].Blocked {
		var id string
		require.NoError(t, json.Unmarshal(b["id"], &id))
		rows[id] = b
	}
	require.Contains(t, rows, "c2", "stdout: %s", stdout)
	assert.JSONEq(t, `true`, string(rows["c2"]["moot"]), "stdout: %s", stdout)
	assert.NotContains(t, rows["c2"], "not_actionable")
	assert.Empty(t, status.Milestones[0].Operator.Conditions)
}

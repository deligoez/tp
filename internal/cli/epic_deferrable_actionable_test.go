package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deferrableConditionGraph: on M, k is a condition whose only dependent is
// e, deferrable in M's gate; dc is a deferrable condition; free is a
// condition nothing depends on; w is the gate's work member.
const deferrableConditionGraph = `{"version": 1, "milestones": [["M"]],
  "gates": {"M": {"members": ["w"], "deferrable": ["e", "dc"]}},
  "epics": [
    {"id": "w", "title": "W", "milestone": "M", "root": true},
    {"id": "k", "title": "K", "milestone": "M", "condition": true, "acceptance": "the board says yes", "root": true},
    {"id": "e", "title": "E", "milestone": "M", "depends_on": [{"id": "k", "why": "e ships once the board agrees"}]},
    {"id": "free", "title": "Free", "milestone": "M", "condition": true, "acceptance": "x", "root": true},
    {"id": "dc", "title": "DC", "milestone": "M", "condition": true, "acceptance": "x", "root": true}
  ]}`

// TestEpicReady_AConditionOnlyDeferrableWorkWaitsOnIsNotActionable is
// spec/1.8.0.md §7: tp epic ready's operator conditions hold free alone; with
// --all, k is not actionable for its dependents, since its only dependent is
// deferrable, and dc for being deferrable itself. The export's items agree.
func TestEpicReady_AConditionOnlyDeferrableWorkWaitsOnIsNotActionable(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, deferrableConditionGraph)
	conditions := func(args ...string) map[string][]string {
		stdout, stderr, code := runTP(t, dir, append([]string{"epic", "ready"}, args...)...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var r struct {
			Operator struct {
				Conditions []struct {
					ID            string   `json:"id"`
					NotActionable []string `json:"not_actionable"`
				} `json:"conditions"`
			} `json:"operator"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
		out := map[string][]string{}
		for _, c := range r.Operator.Conditions {
			out[c.ID] = c.NotActionable
		}
		return out
	}
	assert.Equal(t, map[string][]string{"free": nil}, conditions())
	assert.Equal(t, map[string][]string{"free": nil, "k": {"dependents"}, "dc": {"deferrable"}}, conditions("--all"))

	items, _ := exportEpicItems(t, dir)
	for _, id := range []string{"k", "dc"} {
		require.NotNil(t, items[id].Actionable, id)
		assert.False(t, *items[id].Actionable, "%s is not actionable in the export", id)
	}
}

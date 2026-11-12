package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicScopeGraph is a synthetic two-track graph: m1's gate holds a1, which
// reaches a0 of m0 and b0 of the other track's n0; a2 of m1 is outside it.
const epicScopeGraph = `{"version": 1, "milestones": [["m0", "m1"], ["n0"]],
  "gates": {"m1": {"members": ["a1"], "threshold": "ten users"}},
  "epics": [
    {"id": "a0", "title": "A0", "milestone": "m0", "root": true, "estimate_minutes": 30},
    {"id": "a1", "title": "A1", "milestone": "m1", "estimate_minutes": 60, "depends_on": [
      {"id": "a0", "why": "needs it"}, {"id": "b0", "why": "needs it"}]},
    {"id": "a2", "title": "A2", "milestone": "m1", "root": true, "estimate_minutes": 10},
    {"id": "b0", "title": "B0", "milestone": "n0", "root": true, "estimate_minutes": 90}
  ]}`

// TestEpicScope_Payload: the full payload carries every part of the scope,
// the closure per milestone with the other track marked, and m1's critical
// path in tp epic status's shape.
func TestEpicScope_Payload(t *testing.T) {
	stdout, stderr, code := runTP(t, epicGraphDir(t, epicScopeGraph), "epic", "scope", "m1")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	for _, key := range []string{"milestone", "fingerprint", "members", "members_implicit", "closure", "shrink",
		"outside", "no_dependents", "deferrable_in_closure", "unneeded_earlier", "critical_path"} {
		assert.Contains(t, out, key)
	}
	assert.JSONEq(t, `["a1"]`, string(out["members"]))
	assert.JSONEq(t, `[
	  {"milestone": "m0", "track": 0, "size": 1, "epics": [{"id": "a0"}]},
	  {"milestone": "m1", "track": 0, "size": 1, "epics": [{"id": "a1"}]},
	  {"milestone": "n0", "track": 1, "cross_track": true, "size": 1, "epics": [{"id": "b0"}]}
	]`, string(out["closure"]))
	assert.JSONEq(t, `[{"id": "a2"}]`, string(out["outside"]))
	assert.JSONEq(t, `["a2"]`, string(out["no_dependents"]))
	assert.JSONEq(t, `{"by_count": ["a0", "a1"], "by_estimate": {"chain": ["b0", "a1"], "estimate_minutes": 150}}`,
		string(out["critical_path"]))
}


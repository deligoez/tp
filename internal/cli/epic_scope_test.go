package cli_test

import (
	"encoding/json"
	"fmt"
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
// the fingerprint first, the closure per milestone with the other track
// marked, and m1's critical path in tp epic status's shape.
func TestEpicScope_Payload(t *testing.T) {
	stdout, stderr, code := runTP(t, epicGraphDir(t, epicScopeGraph), "epic", "scope", "m1")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	for _, key := range []string{"milestone", "fingerprint", "members", "members_implicit", "member_count", "closure", "shrink",
		"outside", "deferrable_in_closure", "unneeded_earlier", "critical_path"} {
		assert.Contains(t, out, key)
	}
	assert.Regexp(t, `^\{\s*"fingerprint"`, stdout, "the fingerprint comes first")
	assert.JSONEq(t, `["a1"]`, string(out["members"]))
	assert.JSONEq(t, `1`, string(out["member_count"]))
	assert.JSONEq(t, `null`, string(out["purpose"]), "an unset purpose prints null")
	assert.JSONEq(t, `[
	  {"milestone": "m0", "track": 0, "size": 1, "epics": [{"id": "a0"}]},
	  {"milestone": "m1", "track": 0, "size": 1, "epics": [{"id": "a1"}]},
	  {"milestone": "n0", "track": 1, "cross_track": true, "size": 1, "epics": [{"id": "b0"}]}
	]`, string(out["closure"]))
	assert.JSONEq(t, `[{"id": "a2", "no_dependents": true}]`, string(out["outside"]))
	assert.JSONEq(t, `[]`, string(out["deferrable_in_closure"]))
	assert.JSONEq(t, `{"by_count": ["a0", "a1"], "by_estimate": {"chain": ["b0", "a1"], "estimate_minutes": 150}}`,
		string(out["critical_path"]))
}

// TestEpicScope_Compact: --compact keeps the fingerprint first, the members
// with their mode and count, the closure's sizes, the shrink rows without
// leaves and the critical path by count; an implicit gate leaves its member
// list out and keeps the count.
func TestEpicScope_Compact(t *testing.T) {
	dir := epicGraphDir(t, epicScopeGraph)
	full, _, code := runTP(t, dir, "epic", "scope", "m1")
	require.Equal(t, 0, code)
	var fullOut struct {
		Fingerprint string `json:"fingerprint"`
	}
	require.NoError(t, json.Unmarshal([]byte(full), &fullOut))

	stdout, stderr, code := runTP(t, dir, "--compact", "epic", "scope", "m1")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.JSONEq(t, `{"fingerprint": "`+fullOut.Fingerprint+`", "milestone": "m1", "purpose": null, "members": ["a1"],
	  "members_implicit": false, "member_count": 1,
	  "closure": [{"milestone": "m0", "track": 0, "size": 1}, {"milestone": "m1", "track": 0, "size": 1},
	    {"milestone": "n0", "track": 1, "cross_track": true, "size": 1}],
	  "shrink": [{"epic": "a1", "member": true, "size": 3}],
	  "critical_path": {"by_count": ["a0", "a1"]}}`, stdout)
	assert.Regexp(t, `^\{\s*"fingerprint"`, stdout, "the fingerprint comes first")

	stdout, stderr, code = runTP(t, dir, "--compact", "epic", "scope", "n0")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var implicit map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &implicit))
	assert.NotContains(t, implicit, "members", "an implicit gate's member list is left out")
	assert.JSONEq(t, `true`, string(implicit["members_implicit"]))
	assert.JSONEq(t, `1`, string(implicit["member_count"]))
}

// epicScopeImplicitGraph is a synthetic graph whose m1 gate names no members
// and lists d1 deferrable, which the member b1 requires anyway, under a
// written purpose.
const epicScopeImplicitGraph = `{"version": 1, "milestones": [["m0", "m1"]],
  "gates": {"m1": {"members": [], "deferrable": ["d1"], "threshold": "ten users", "purpose": "prove the import path"}},
  "epics": [
    {"id": "a0", "title": "A0", "milestone": "m0", "root": true},
    {"id": "b1", "title": "B1", "milestone": "m1", "depends_on": [
      {"id": "a0", "why": "needs it"}, {"id": "d1", "why": "needs it"},
      {"any_of": ["b2", "b3"], "why": "needs one"}]},
    {"id": "b2", "title": "B2", "milestone": "m1", "depends_on": [{"id": "b4", "why": "needs it"}]},
    {"id": "b3", "title": "B3", "milestone": "m1", "root": true},
    {"id": "b4", "title": "B4", "milestone": "m0", "root": true},
    {"id": "d1", "title": "D1", "milestone": "m1", "root": true}
  ]}`

// TestEpicScope_CompactIsASubsetOfFull: every key path the compact payload
// prints is in the full payload with an equal value, arrays element by
// element, for an explicit and an implicit gate; compact keeps a non-empty
// deferrable_in_closure.
func TestEpicScope_CompactIsASubsetOfFull(t *testing.T) {
	for name, graph := range map[string]string{"explicit": epicScopeGraph, "implicit": epicScopeImplicitGraph} {
		t.Run(name, func(t *testing.T) {
			dir := epicGraphDir(t, graph)
			fullOut, stderr, code := runTP(t, dir, "epic", "scope", "m1")
			require.Equal(t, 0, code, "stderr: %s", stderr)
			compactOut, stderr, code := runTP(t, dir, "--compact", "epic", "scope", "m1")
			require.Equal(t, 0, code, "stderr: %s", stderr)
			var full, compact any
			require.NoError(t, json.Unmarshal([]byte(fullOut), &full))
			require.NoError(t, json.Unmarshal([]byte(compactOut), &compact))
			assertJSONSubset(t, "$", compact, full)
			if name == "implicit" {
				assert.Contains(t, compact, "deferrable_in_closure")
				assert.Equal(t, "prove the import path", compact.(map[string]any)["purpose"], "compact keeps a written purpose")
			}
		})
	}
}

// assertJSONSubset checks that every key path of sub is in full with an
// equal value: objects key by key, arrays element by element at equal
// length, scalars equal.
func assertJSONSubset(t *testing.T, path string, sub, full any) {
	t.Helper()
	switch s := sub.(type) {
	case map[string]any:
		f, ok := full.(map[string]any)
		if !assert.True(t, ok, "%s is an object in full", path) {
			return
		}
		for key, value := range s {
			fv, ok := f[key]
			if assert.True(t, ok, "%s.%s is in full", path, key) {
				assertJSONSubset(t, path+"."+key, value, fv)
			}
		}
	case []any:
		f, ok := full.([]any)
		if !assert.True(t, ok, "%s is an array in full", path) || !assert.Len(t, f, len(s), path) {
			return
		}
		for i := range s {
			assertJSONSubset(t, fmt.Sprintf("%s[%d]", path, i), s[i], f[i])
		}
	default:
		assert.Equal(t, full, sub, path)
	}
}

// TestEpicScope_UnknownMilestoneExits4 answers a milestone no track lists as
// tp epic gate does.
func TestEpicScope_UnknownMilestoneExits4(t *testing.T) {
	_, stderr, code := runTP(t, epicGraphDir(t, epicScopeGraph), "epic", "scope", "zz")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "zz")
}

// TestEpicScope_NoGraphExits3: with no graph file the loader exits 3.
func TestEpicScope_NoGraphExits3(t *testing.T) {
	_, _, code := runTP(t, epicGraphDir(t, ""), "epic", "scope", "m1")
	assert.Equal(t, 3, code)
}

// TestEpicScope_RunsUnattended: the command is read-only, so TP_UNATTENDED
// does not fence it, and it prints the same payload.
func TestEpicScope_RunsUnattended(t *testing.T) {
	dir := epicGraphDir(t, epicScopeGraph)
	unattended, stderr, code := runTPFence(t, dir, true, "epic", "scope", "m1")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	attended, _, code := runTPFence(t, dir, false, "epic", "scope", "m1")
	require.Equal(t, 0, code)
	assert.JSONEq(t, attended, unattended)
}

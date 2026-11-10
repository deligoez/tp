package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anyOfRemoveGraph is track [A] with root epics x, y and z, x done when
// xDone, and w whose depends_on is the JSON array deps.
func anyOfRemoveGraph(deps string, xDone bool) string {
	done := ""
	if xDone {
		done = `, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "x shipped"}`
	}
	return `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "w", "title": "W", "milestone": "A", "depends_on": ` + deps + `},
    {"id": "x", "title": "X", "milestone": "A", "root": true` + done + `},
    {"id": "y", "title": "Y", "milestone": "A", "root": true},
    {"id": "z", "title": "Z", "milestone": "A", "root": true}
  ]
}
`
}

// removeX runs tp epic remove x --force in a graph built from deps and
// returns w's stored depends_on afterwards.
func removeX(t *testing.T, deps string, xDone bool) []map[string]any {
	t.Helper()
	dir := epicGraphDir(t, anyOfRemoveGraph(deps, xDone))
	_, stderr, code := runTP(t, dir, "epic", "remove", "x", "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	return storedDeps(t, dir, "w")
}

// TestEpicRemove_AGroupMemberIsADependencyNeedingForce (spec/1.7.0.md §7):
// membership of w's group makes x one of w's dependencies, so a remove
// without --force is refused, names w, and writes nothing.
func TestEpicRemove_AGroupMemberIsADependencyNeedingForce(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfRemoveGraph(`[{"any_of": ["x", "y"], "why": "either opens the API"}]`, false))
	before := readEpicGraphFile(t, dir)
	_, stderr, code := runTP(t, dir, "epic", "remove", "x")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `epics w depend on \"x\"`)
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

// TestEpicRemove_ForceOnADoneMemberDropsItsGroup: a done x satisfied the
// group, so removing it drops the whole group rather than leaving w waiting
// on y.
func TestEpicRemove_ForceOnADoneMemberDropsItsGroup(t *testing.T) {
	t.Parallel()
	assert.Empty(t, removeX(t, `[{"any_of": ["x", "y", "z"], "why": "any opens the API"}]`, true))
}

// TestEpicRemove_ForceOnAnUndoneMemberLeavesTheGroup: an undone x leaves the
// group, which keeps its other members and its why.
func TestEpicRemove_ForceOnAnUndoneMemberLeavesTheGroup(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []map[string]any{anyOfEntry("any opens the API", "y", "z")},
		removeX(t, `[{"any_of": ["x", "y", "z"], "why": "any opens the API"}]`, false))
}


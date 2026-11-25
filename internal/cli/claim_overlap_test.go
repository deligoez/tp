package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overlapRepo is claimsRepo with t4 added and touches set: t2 overlaps t1,
// t4 overlaps neither. Nothing is claimed yet.
func overlapRepo(t *testing.T) string {
	t.Helper()
	dir := claimsRepo(t)
	addTaskWithEstimate(t, dir, "t4", 9)
	for id, touches := range map[string]string{"t1": `["go.mod"]`, "t2": `["go.mod","go.sum"]`, "t4": `["docs/"]`} {
		_, stderr, code := runTP(t, dir, "set", id, "touches="+touches)
		require.Equal(t, 0, code, stderr)
	}
	return dir
}

// holdT1 claims t1 for an agent spawn, the claim tp brief screens against.
func holdT1(t *testing.T, dir string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A", "--session", "sessC")
	require.Equal(t, 0, code, stderr)
}

// TestNext_PassesOverATaskOverlappingAClaim: tp next claims under the touches
// rule tp brief uses. While t1 is held, t2 (first by priority) overlaps it,
// so tp next claims t4 and leaves t2 open.
func TestNext_PassesOverATaskOverlappingAClaim(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	holdT1(t, dir)

	stdout, stderr, code := runTP(t, dir, "next")
	require.Equal(t, 0, code, stderr)
	var next struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &next), stdout)
	assert.Equal(t, "t4", next.Task.ID, "tp next passes over t2")
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}

// TestNext_ReportsNoneWhenEveryReadyTaskOverlapsAClaim: with t4 done, the
// only ready task t2 overlaps the held t1, so tp next claims nothing, exits
// 4 and names t1 as the agent-held task.
func TestNext_ReportsNoneWhenEveryReadyTaskOverlapsAClaim(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	_, stderr, code := runTP(t, dir, "done", "t4", "t4 was finished before t1 was claimed", "--gate-passed")
	require.Equal(t, 0, code, stderr)
	holdT1(t, dir)

	stdout, _, code := runTP(t, dir, "next")
	assert.Equal(t, 4, code, "no task is claimable: %s", stdout)
	assert.Contains(t, stdout, `"t1"`, "the holder is named")
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}

// TestClaim_RefusesATaskOverlappingAClaim: tp claim t2 refuses with exit 4
// and the message tp brief gives, naming t1, and leaves t2 open.
func TestClaim_RefusesATaskOverlappingAClaim(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	holdT1(t, dir)

	_, stderr, code := runTP(t, dir, "claim", "t2")
	assert.Equal(t, 4, code, stderr)
	var refusal struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(stderr), &refusal), stderr)
	assert.Contains(t, refusal.Error, `task "t2" is not handed out while task "t1" is claimed`)
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}

// TestClaim_RefusesASpawnClaimOverlappingAClaim: the Agent hook's claim is
// screened too, so two briefs read before either claim cannot both start.
func TestClaim_RefusesASpawnClaimOverlappingAClaim(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	holdT1(t, dir)

	_, stderr, code := runTP(t, dir, "claim", "t2", "--spawn", "toolu_B", "--session", "sessC")
	assert.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, `\"t1\" is claimed`)
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}


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

// assertNamesTheOverlap runs tp <cmd> with t4 done and t1 held, so the only
// ready task t2 overlaps t1, and checks the exit-4 payload lists t2 under
// overlapping, held by t1, and that the message counts it.
func assertNamesTheOverlap(t *testing.T, cmd string) {
	t.Helper()
	dir := overlapRepo(t)
	_, stderr, code := runTP(t, dir, "done", "t4", "t4 was finished before t1 was claimed", "--gate-passed")
	require.Equal(t, 0, code, stderr)
	holdT1(t, dir)

	stdout, _, code := runTP(t, dir, cmd)
	require.Equal(t, 4, code, stdout)
	var res struct {
		Message     string `json:"message"`
		Overlapping []struct {
			ID     string `json:"id"`
			HeldBy string `json:"held_by"`
		} `json:"overlapping"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	require.Len(t, res.Overlapping, 1, stdout)
	assert.Equal(t, "t2", res.Overlapping[0].ID)
	assert.Equal(t, "t1", res.Overlapping[0].HeldBy)
	assert.Contains(t, res.Message, "1 overlapping a held task", res.Message)
}

// TestNext_NamesTheTaskItPassesOverForAnOverlap: when the only ready task
// overlaps a held one, the not-ready payload lists it under overlapping with
// the task that holds its files, and the message counts it.
func TestNext_NamesTheTaskItPassesOverForAnOverlap(t *testing.T) {
	t.Parallel()
	assertNamesTheOverlap(t, "next")
}

// TestBrief_NamesTheTaskItPassesOverForAnOverlap: tp brief with no id gives
// tp next's not-ready payload, so the only ready task, overlapping a held
// one, is listed under overlapping with its holder rather than counted as
// blocked.
func TestBrief_NamesTheTaskItPassesOverForAnOverlap(t *testing.T) {
	t.Parallel()
	assertNamesTheOverlap(t, "brief")
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

// TestClaim_BatchFailsOnlyTheOverlappingTask: tp claim t2 t4 claims t4 and
// lists t2 under failed with the overlap message.
func TestClaim_BatchFailsOnlyTheOverlappingTask(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	holdT1(t, dir)

	stdout, stderr, code := runTP(t, dir, "claim", "t2", "t4")
	require.Equal(t, 0, code, stderr)
	var res struct {
		Claimed []string `json:"claimed"`
		Failed  []struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	assert.Equal(t, []string{"t4"}, res.Claimed)
	require.Len(t, res.Failed, 1)
	assert.Equal(t, "t2", res.Failed[0].ID)
	assert.Contains(t, res.Failed[0].Error, `"t1"`)
}

// TestClaim_AllReadySkipsTheOverlappingTask: --all-ready claims the ready
// tasks the overlap rule hands out, and leaves t2 open.
func TestClaim_AllReadySkipsTheOverlappingTask(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	holdT1(t, dir)

	_, stderr, code := runTP(t, dir, "claim", "--all-ready")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "wip", taskState(t, dir, "t4")["status"])
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}

// TestClaim_UnheldWipAndTouchlessTasksAreNotScreened: a wip task nobody
// holds (a plain tp claim) screens nothing, and a task without touches is
// claimable beside a held one.
func TestClaim_UnheldWipAndTouchlessTasksAreNotScreened(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	_, stderr, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t2")
	require.Equal(t, 0, code, "an unheld wip task screens nothing: %s", stderr)

	dir = overlapRepo(t)
	holdT1(t, dir)
	_, stderr, code = runTP(t, dir, "set", "t2", "touches=[]")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t2")
	assert.Equal(t, 0, code, "a task without touches is claimable: %s", stderr)
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A filled pool (spec/backlog/filled-pool.md): a coordinator session claims
// through spawns, and tp pool says which of its slots are free and what
// could fill them.

const poolHook = "hooks/stop-fill-pool.sh"

// poolRepo is claimsRepo (t1, t2 ready, t3 after t1) with t4-t7 ready too
// and run_max_parallel 3.
func poolRepo(t *testing.T) string {
	t.Helper()
	dir := claimsRepo(t)
	for _, id := range []string{"t4", "t5", "t6", "t7"} {
		addTaskWithEstimate(t, dir, id, 5)
	}
	_, stderr, code := runTP(t, dir, "set", "--workflow", "run_max_parallel=3")
	require.Equal(t, 0, code, stderr)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "more")
	return dir
}

type poolID struct {
	ID string `json:"id"`
}

type poolOut struct {
	Coordinator bool     `json:"coordinator"`
	Cap         int      `json:"cap"`
	Running     []poolID `json:"running"`
	Parked      []poolID `json:"parked"`
	Unresolved  int      `json:"unresolved"`
	Orphans     []poolID `json:"orphans"`
	Respawn     []poolID `json:"respawn"`
	Unmerged    []poolID `json:"unmerged"`
	Free        int      `json:"free"`
	Ready       []poolID `json:"ready"`
	Ask         bool     `json:"ask"`
	Reason      string   `json:"reason"`
}

func pool(t *testing.T, dir string, args ...string) poolOut {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"pool"}, args...)...)
	require.Equal(t, 0, code, stderr)
	var p poolOut
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), stdout)
	return p
}

func idsOf(list []poolID) []string {
	out := []string{}
	for _, x := range list {
		out = append(out, x.ID)
	}
	return out
}

func spawnClaim(t *testing.T, dir, id, spawn, session string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "claim", id, "--spawn", spawn, "--session", session)
	require.Equal(t, 0, code, stderr)
}

func TestPoolClassifiesTheSessionsClaims(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	spawnClaim(t, dir, "t2", "toolu_B", "sessC")
	spawnClaim(t, dir, "t4", "toolu_D", "sessC")
	_, _, code := runTP(t, dir, "set", "t2", "waiting=a long run")
	require.Equal(t, 0, code)

	p := pool(t, dir, "--session", "sessC", "--live", "toolu_A")
	assert.True(t, p.Coordinator)
	assert.Equal(t, 3, p.Cap)
	assert.Equal(t, []string{"t1"}, idsOf(p.Running), "a live spawn runs")
	assert.Equal(t, []string{"t2"}, idsOf(p.Parked), "waiting parks")
	assert.Equal(t, []string{"t4"}, idsOf(p.Orphans), "a gone spawn without waiting is an orphan")
	assert.Equal(t, 1, p.Free, "running and parked hold two of three slots; the orphan holds none")
	assert.Equal(t, []string{"t5", "t6", "t7"}, idsOf(p.Ready), "t3 waits on wip t1")
	assert.True(t, p.Ask)
	assert.Contains(t, p.Reason, "`tp brief t5 --file spec.tasks.json`")
	assert.NotContains(t, p.Reason, "tp brief t6", "only as many ready tasks as free slots")
	assert.Contains(t, p.Reason, "`tp claim t4 --takeover --file spec.tasks.json`")
	assert.Contains(t, p.Reason, `waiting=`, "the reason names the way to hold a task back")
}

func TestPoolDoesNotAskAFullPool(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	for _, id := range []string{"t1", "t2", "t4"} {
		spawnClaim(t, dir, id, "toolu_"+id, "sessC")
	}
	p := pool(t, dir, "--session", "sessC", "--live", "toolu_t1,toolu_t2", "--live", "toolu_t4")
	assert.Equal(t, 0, p.Free)
	assert.False(t, p.Ask)
	assert.Empty(t, p.Reason)
}

func TestPoolIgnoresASessionThatNeverClaimed(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessOther")
	p := pool(t, dir, "--session", "sessC")
	assert.False(t, p.Coordinator)
	assert.False(t, p.Ask, "an ordinary session in a tp project is never asked")
	assert.Equal(t, 0, p.Cap)
}

func TestPoolCountsOnlyItsOwnSessionsClaims(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessOther")
	spawnClaim(t, dir, "t2", "toolu_B", "sessC")
	_, _, code := runTP(t, dir, "claim", "t4")
	require.Equal(t, 0, code, "a plain claim by hand carries no session")

	p := pool(t, dir, "--session", "sessC", "--live", "toolu_B")
	assert.Empty(t, p.Orphans, "another session's claim and a hand claim are not this coordinator's orphans")
	assert.Equal(t, 2, p.Free)
}

func TestPoolStillCountsACoordinatorWhoseClaimsAreAllDone(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	p := pool(t, dir, "--session", "sessC")
	assert.True(t, p.Coordinator, "a session that has held a claim stays a coordinator")
	assert.Equal(t, 3, p.Free)
	assert.Contains(t, idsOf(p.Ready), "t3", "t3 is ready once t1 is done")
}

func TestPoolCountsAnUnresolvedAgentAndCallsNoClaimAnOrphan(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	p := pool(t, dir, "--session", "sessC", "--unresolved", "1")
	assert.Empty(t, p.Orphans, "an agent whose spawn is unknown may be t1's")
	assert.Equal(t, 2, p.Free, "and it holds a slot")
}

func TestPoolNamesATakenOverClaimForRespawn(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	_, stderr, code := runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	p := pool(t, dir, "--session", "sessC")
	assert.Equal(t, []string{"t1"}, idsOf(p.Respawn))
	assert.Empty(t, p.Orphans)
	assert.Contains(t, p.Reason, "Claim t1 has no agent")
}

func TestPoolReadsOtherWorktreeCopies(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	spawnClaim(t, dir, "t2", "toolu_B", "sessC")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, wt, "set", "t2", "waiting=a long run")
	require.Equal(t, 0, code, stderr)

	p := pool(t, dir, "--session", "sessC")
	assert.Equal(t, []string{"t1"}, idsOf(p.Unmerged), "closed in a worktree, not merged yet")
	assert.Equal(t, []string{"t2"}, idsOf(p.Parked), "a worktree agent parked it on its own copy")
	assert.Empty(t, p.Orphans)
}

func TestPoolLeavesHeldTasksOutOfReady(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	for _, id := range []string{"t2", "t4", "t5", "t6", "t7"} {
		_, _, code := runTP(t, dir, "set", id, "waiting=the release tag")
		require.Equal(t, 0, code)
	}
	p := pool(t, dir, "--session", "sessC", "--live", "toolu_A")
	assert.Empty(t, p.Ready)
	assert.False(t, p.Ask, "a hold is the coordinator's on-disk answer, so the pool stops asking")
}

func TestPoolSkipsAFileItsEpicBlocks(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "wip", "open")
	raw := taskFileBytes(t, dir)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(raw, &tf))
	tf["tasks"].([]any)[0].(map[string]any)["claimed_by"] = map[string]any{"session": "sessC", "spawn": "toolu_A"}
	raw, err := json.Marshal(tf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), raw, 0o600))
	gitIn(t, dir, "init", "-q")

	p := pool(t, dir, "--session", "sessC", "--live", "toolu_A")
	assert.True(t, p.Coordinator)
	assert.Empty(t, p.Ready, "t2 is open, but its file's epic is blocked")
}

// poolStopPayload is a Stop hook input for session sessC in dir whose transcript
// directory holds one running agent's meta.json with spawn toolu_A.
func poolStopPayload(t *testing.T, dir string, active bool) map[string]any {
	t.Helper()
	transcript := filepath.Join(t.TempDir(), "sessC.jsonl")
	subagents := filepath.Join(transcript[:len(transcript)-len(".jsonl")], "subagents")
	require.NoError(t, os.MkdirAll(subagents, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(subagents, "agent-a1b2.meta.json"), []byte(`{"toolUseId":"toolu_A"}`), 0o600))
	return map[string]any{
		"session_id":       "sessC",
		"transcript_path":  transcript,
		"cwd":              dir,
		"hook_event_name":  "Stop",
		"stop_hook_active": active,
		"background_tasks": []any{
			map[string]any{"id": "a1b2", "type": "subagent", "status": "running", "description": "unit"},
			map[string]any{"id": "b9", "type": "shell", "status": "running", "command": "sleep 1"},
		},
	}
}

func TestPoolStopHookAsksACoordinatorToFillFreeSlots(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	spawnClaim(t, dir, "t2", "toolu_B", "sessC")

	stdout, _, code := hookRun(t, poolHook, true, poolStopPayload(t, dir, false))
	require.Equal(t, 0, code)
	var out map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	assert.Equal(t, "block", out["decision"])
	assert.Contains(t, out["reason"], "tp brief t4", "t1 runs through the meta.json spawn, t2 is an orphan, two slots are free")
	assert.Contains(t, out["reason"], "tp claim t2 --takeover")
	assert.NotContains(t, out["reason"], "t1 --takeover", "the live agent's claim is not an orphan")
}

func TestPoolStopHookStaysSilent(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_Z", "sessC")

	cases := map[string]struct {
		payload map[string]any
		withTP  bool
		env     []string
	}{
		"second stop":    {payload: poolStopPayload(t, dir, true), withTP: true},
		"inside a unit":  {payload: poolStopPayload(t, dir, false), withTP: true, env: []string{"TP_UNIT_KIND=implement"}},
		"no tp on PATH":  {payload: poolStopPayload(t, dir, false), withTP: false},
		"not a claimant": {payload: func() map[string]any { p := poolStopPayload(t, dir, false); p["session_id"] = "sessX"; return p }(), withTP: true},
	}
	for name, c := range cases {
		stdout, _, code := hookRun(t, poolHook, c.withTP, c.payload, c.env...)
		assert.Equal(t, 0, code, name)
		assert.Empty(t, stdout, name)
	}
}

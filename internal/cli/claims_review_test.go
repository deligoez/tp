package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Repairs from the v1.29.0 review round: a worktree copy speaks for a claim
// only by what it wrote after the claim.

// TestClaimsCountsAWorktreeAgentParkingOnItsOpenCopy: the coordinator's claim
// is not committed, so an agent in its own worktree sees the task open, and
// its tp set waiting= lands as a hold on that copy, written after the claim.
func TestClaimsCountsAWorktreeAgentParkingOnItsOpenCopy(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	spawnClaim(t, dir, "t1", "toolu_P1", "sessC")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)
	_, stderr, code := runTP(t, wt, "set", "t1", "waiting=a long run")
	require.Equal(t, 0, code, stderr)

	stdout, stderr, code := runTP(t, dir, "claims", "--spawn", "toolu_P1")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"unaccounted":[]`)
}

// TestAWorktreeCloseIsWrittenBesideTheClaim: the agent's branch carries no
// task-file change, so the coordinator's merge cannot meet its uncommitted
// claims, and the close keeps the claim's start.
func TestAWorktreeCloseIsWrittenBesideTheClaim(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	spawnClaim(t, dir, "t1", "toolu_W", "sessC")
	started := taskState(t, dir, "t1")["started_at"]
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	assert.Equal(t, "done", taskState(t, dir, "t1")["status"])
	assert.Equal(t, started, taskState(t, dir, "t1")["started_at"])
	assert.Empty(t, gitRun(t, wt, "status", "--porcelain"), "the worktree's task file is untouched")
}

// TestPoolDoesNotTrustAStaleCloseInAnotherWorktree: a worktree closed t1
// before the coordinator claimed it again; that close says nothing of the
// new claim, which has lost its agent.
func TestPoolDoesNotTrustAStaleCloseInAnotherWorktree(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")

	p := pool(t, dir, "--session", "sessC")
	assert.Empty(t, p.Unmerged)
	assert.Equal(t, []string{"t1"}, idsOf(p.Orphans))
}

func TestPoolCountsALiveAgentWhoseTaskIsClosedInItsWorktree(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	p := pool(t, dir, "--session", "sessC", "--live", "toolu_A")
	assert.Equal(t, []string{"t1"}, idsOf(p.Running), "the agent still runs, so it holds its slot")
	assert.Equal(t, 2, p.Free)
}

func TestPoolDoesNotOfferATaskClosedInAnotherWorktree(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)
	_, stderr, code := runTP(t, wt, "done", "t5", "--gate-passed", "--", "t5 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	p := pool(t, dir, "--session", "sessC", "--live", "toolu_A")
	assert.NotContains(t, idsOf(p.Ready), "t5", "another worktree closed it")
}

func TestPoolDoesNotOfferATaskWhoseTouchesOverlapARunningOne(t *testing.T) {
	t.Parallel()
	dir := poolRepo(t)
	for _, id := range []string{"t1", "t4"} {
		_, stderr, code := runTP(t, dir, "set", id, `touches=["src/shared.go"]`)
		require.Equal(t, 0, code, stderr)
	}
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")

	p := pool(t, dir, "--session", "sessC", "--live", "toolu_A")
	assert.NotContains(t, idsOf(p.Ready), "t4", "tp run would not start it beside t1")
	assert.Contains(t, idsOf(p.Ready), "t2")
}

// TestAgentClaimHookClaimsFromAPastedJSONBrief: tp brief run from a shell
// prints JSON, and a coordinator pastes that whole into the prompt.
func TestAgentClaimHookClaimsFromAPastedJSONBrief(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	brief, stderr, code := runTP(t, dir, "brief", "t1", "--file", "spec.tasks.json")
	require.Equal(t, 0, code, stderr)
	require.Contains(t, brief, `"marker"`, "piped, the brief is JSON")

	_, stderr, code = hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_json", brief))
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"])
}

func TestAgentClaimHookRefusesASpawnWithTwoMarkersClaimingNothing(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, _, code := hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_two", "brief\ntp-task: t1 spec.tasks.json\ntp-task: t2 spec.tasks.json\n"))
	assert.Equal(t, 2, code, "one task per spawn")
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"], "nothing is claimed for a refused spawn")
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}

func TestSubagentStopHookPassesWithATPThatHasNoClaims(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tp"), []byte("#!/bin/sh\n# v1.28.0: claims is an unknown command, help on anything exits 0\n[ \"$1\" = claims ] && { echo 'unknown command \"claims\"' >&2; exit 2; }\nexit 0\n"), 0o700)) //nolint:gosec // a fake tp the hook must run
	transcript := filepath.Join(t.TempDir(), "agent-x.jsonl")
	require.NoError(t, os.WriteFile(transcript[:len(transcript)-len(".jsonl")]+".meta.json", []byte(`{"toolUseId":"toolu_old"}`), 0o600))
	payload := map[string]any{"hook_event_name": "SubagentStop", "stop_hook_active": false, "agent_transcript_path": transcript, "cwd": t.TempDir()}
	stdout, _, code := hookRun(t, agentStopHook, false, payload, "PATH="+bin+":/usr/bin:/bin")
	assert.Equal(t, 0, code)
	assert.Empty(t, stdout, "a tp older than the plugin blocks no stop")
}

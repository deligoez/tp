package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Agent-bound claims (spec/backlog/agent-bound-claims.md): the brief carries
// its task as a marker line, the plugin's PreToolUse(Agent) hook claims it
// for the spawn, and the plugin's SubagentStop hook refuses one stop while a
// task that spawn claimed is neither done nor waiting.

const (
	agentClaimHook = "hooks/pre-tool-use-agent-claim.sh"
	agentStopHook  = "hooks/subagent-stop-claim.sh"
)

// claimsRepo is a git repository holding one committed task file with three
// tasks: t1 and t2 open and ready, t3 depending on t1.
func claimsRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\nContent.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	addTaskWithEstimate(t, dir, "t1", 5)
	addTaskWithEstimate(t, dir, "t2", 5)
	addTask(t, dir, `{"id":"t3","title":"Task t3","depends_on":["t1"],"estimate_minutes":5,"acceptance":"t3 acceptance criteria met","source_sections":["s1"]}`)
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "config", "user.email", "test@test.com")
	gitIn(t, dir, "config", "user.name", "Test")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "seed")
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// claimedBy is the claimed_by object of one task, or nil.
func claimedBy(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	cb, _ := taskState(t, dir, id)["claimed_by"].(map[string]any)
	return cb
}

func TestBriefCarriesItsTaskMarker(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)

	stdout, stderr, code := runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	var brief map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Equal(t, "tp-task: t1 spec.tasks.json", brief["marker"])

	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	stdout, stderr, code = runTP(t, sub, "--file", "../spec.tasks.json", "brief", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Equal(t, "tp-task: t1 spec.tasks.json", brief["marker"], "the path is relative to the repository root, not the working directory")

	// An id the spawn hook would not take as a marker gets none, rather than
	// a marker the hook silently skips.
	addTask(t, dir, `{"id":"_odd","title":"Odd","depends_on":[],"estimate_minutes":5,"acceptance":"odd acceptance criteria met","source_sections":["s1"]}`)
	stdout, stderr, code = runTP(t, dir, "brief", "_odd")
	require.Equal(t, 0, code, stderr)
	brief = map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.NotContains(t, brief, "marker")
}

// A brief an Agent spawn claims from carries a fresh claim token, on a line
// of its own and in the close recipe; a brief inside tp run carries none,
// since the hook does not claim for a unit.
func TestBriefCarriesAClaimToken(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)

	stdout, stderr, code := runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	var brief map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	token, _ := brief["claim_token"].(string)
	require.Regexp(t, `^[a-z0-9]{8,32}$`, token)
	assert.Contains(t, brief["close"], "--claim-token "+token+" -- ", "the worktree close command names the token before the reason")

	stdout, stderr, code = runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.NotEqual(t, token, brief["claim_token"], "every brief mints its own token")

	stdout, stderr, code = runTPEnv(t, dir, []string{"TP_UNIT_KIND=implement"}, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	brief = map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.NotContains(t, brief, "claim_token", "a tp run unit's brief carries no token")
}

// The spawn hook records the brief's claim token in the claim it makes, from
// the plain line and from a pasted JSON brief; a marker without one claims
// as before.
func TestAgentClaimHookRecordsTheClaimToken(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, stderr, code := hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_T1", "brief\ntp-task: t1 spec.tasks.json\ntp-claim-token: k7f2ab01\n"))
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "k7f2ab01", claimedBy(t, dir, "t1")["token"])

	pasted := "{\n  \"marker\": \"tp-task: t2 spec.tasks.json\",\n  \"claim_token\": \"q9q9q9q9\",\n  \"identity\": \"x\"\n}"
	_, stderr, code = hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_T2", pasted))
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "q9q9q9q9", claimedBy(t, dir, "t2")["token"])

	_, _, code = hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_T4", "tp-task: t4 spec.tasks.json\ntp-claim-token: bad token\n"))
	assert.Equal(t, 2, code, "a token tp would not record refuses the spawn")
}

// A takeover frees the claim's token with its spawn.
func TestTakeoverClearsTheClaimToken(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, stderr, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A", "--session", "sessX", "--claim-token", "aaaa1111")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "aaaa1111", claimedBy(t, dir, "t1")["token"])
	_, stderr, code = runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, map[string]any{"session": "sessX", "retired": []any{"aaaa1111"}}, claimedBy(t, dir, "t1"), "the session stays; the spawn goes and its token is retired")

	// A spawn from the brief the takeover replaced reuses its token: refused,
	// so the two agents cannot share one token.
	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_A2", "--session", "sessX", "--claim-token", "aaaa1111")
	assert.Equal(t, 4, code, "a replaced brief's token is refused: %s", stderr)
	assert.Contains(t, stderr, "tp brief t1")
	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_A2", "--session", "sessX", "--claim-token", "bbbb2222")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	_, _, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_A3", "--claim-token", "aaaa1111")
	assert.Equal(t, 4, code, "every replaced token stays refused")

	// An unclaim retires the token too: the brief it freed is not reused.
	_, stderr, code = runTP(t, dir, "claim", "t2", "--spawn", "toolu_C", "--claim-token", "cccc3333")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "unclaim", "t2")
	require.Equal(t, 0, code, stderr)
	_, _, code = runTP(t, dir, "claim", "t2", "--spawn", "toolu_C2", "--claim-token", "cccc3333")
	assert.Equal(t, 4, code, "an unclaimed brief's token is refused")

	// A reopen retires the token of the claim that closed the task.
	_, stderr, code = runTP(t, dir, "claim", "t2", "--spawn", "toolu_D", "--claim-token", "dddd4444")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "done", "t2", "--", "t2 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "reopen", "t2")
	require.Equal(t, 0, code, stderr)
	_, _, code = runTP(t, dir, "claim", "t2", "--spawn", "toolu_D2", "--claim-token", "dddd4444")
	assert.Equal(t, 4, code, "a reopened task's old token is refused")

	_, _, code = runTP(t, dir, "claim", "t2", "--spawn", "toolu_B", "--claim-token", "no good")
	assert.Equal(t, 2, code, "a token tp would not record is a usage error")
	_, _, code = runTP(t, dir, "claim", "t2", "--claim-token", "aaaa1111")
	assert.Equal(t, 2, code, "--claim-token needs --spawn")
}

func TestClaimSpawnClaimsAnOpenTaskForTheSpawn(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)

	_, stderr, code := runTPEnv(t, dir, []string{"CLAUDE_CODE_SESSION_ID=not-read"}, "claim", "t1", "--spawn", "toolu_A1", "--session", "sess-1")
	require.Equal(t, 0, code, stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, "wip", task["status"])
	assert.Equal(t, "claimed", task["duration_source"])
	assert.Equal(t, map[string]any{"session": "sess-1", "spawn": "toolu_A1"}, task["claimed_by"])

	_, stderr, code = runTPEnv(t, dir, []string{"CLAUDE_CODE_SESSION_ID=not-read"}, "claim", "t2", "--spawn", "toolu_A1")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, map[string]any{"spawn": "toolu_A1"}, claimedBy(t, dir, "t2"), "no --session, no session: tp never reads it from the environment")
}

func TestClaimSpawnRebindsAWipTaskAndKeepsItsStart(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, _, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code)
	started := taskState(t, dir, "t1")["started_at"]

	_, stderr, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A2")
	require.Equal(t, 0, code, stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, started, task["started_at"], "a rebind keeps the coordinator's claim time")
	assert.Equal(t, "toolu_A2", claimedBy(t, dir, "t1")["spawn"])

	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_A2")
	assert.Equal(t, 0, code, "the same spawn may claim again: %s", stderr)
}

func TestClaimSpawnRefuses(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, _, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A3", "--session", "sess-0")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "done", "t2", "--gate-passed", "--", "t2 acceptance criteria met")
	require.Equal(t, 0, code)

	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"held by another spawn", []string{"claim", "t1", "--spawn", "toolu_B3"}, 4},
		{"done", []string{"claim", "t2", "--spawn", "toolu_B3"}, 4},
		{"blocked", []string{"claim", "t3", "--spawn", "toolu_B3"}, 4},
		{"unknown id", []string{"claim", "nope", "--spawn", "toolu_B3"}, 4},
		{"spawn shape", []string{"claim", "t3", "--spawn", "toolu;rm"}, 2},
		{"session shape", []string{"claim", "t1", "--spawn", "toolu_A3", "--session", "a b"}, 2},
		{"empty session", []string{"claim", "t1", "--spawn", "toolu_A3", "--session", ""}, 2},
		{"one id only", []string{"claim", "t1", "t3", "--spawn", "toolu_B3"}, 2},
		{"session needs spawn", []string{"claim", "t1", "--session", "s"}, 2},
		{"takeover takes no spawn", []string{"claim", "t1", "--spawn", "toolu_B3", "--takeover"}, 2},
		{"takeover takes one id", []string{"claim", "t1", "t3", "--takeover"}, 2},
		{"takeover of a task not wip", []string{"claim", "t2", "--takeover"}, 4},
	} {
		_, stderr, got := runTP(t, dir, tc.args...)
		assert.Equal(t, tc.code, got, "%s: %s", tc.name, stderr)
	}
	assert.Equal(t, "toolu_A3", claimedBy(t, dir, "t1")["spawn"], "a refused claim leaves the holder")

	_, stderr, _ := runTP(t, dir, "claim", "t1", "--spawn", "toolu_B3")
	assert.Contains(t, stderr, "tp claim t1 --takeover", "the refusal names the takeover")
	assert.Contains(t, stderr, "tp unclaim t1", "and the unclaim")

	// The takeover frees the claim from its dead spawn and keeps the rest, so
	// the next spawn from the brief binds as for a coordinator's claim.
	started := taskState(t, dir, "t1")["started_at"]
	_, stderr, code = runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, "wip", task["status"])
	assert.Equal(t, started, task["started_at"], "a takeover keeps the start")
	assert.Equal(t, map[string]any{"session": "sess-0"}, task["claimed_by"], "a takeover clears only the spawn")

	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_B3")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "toolu_B3", claimedBy(t, dir, "t1")["spawn"])
	assert.Equal(t, started, taskState(t, dir, "t1")["started_at"])
}

// A task an agent spawn holds is that agent's: tp brief and tp next pass
// over it to the next ready task, and when agents hold all that could run,
// tp next exits 4 naming them and tp resume names the pool, not a blocker.
func TestNextPassesOverAnAgentHeldTask(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	spawnClaim(t, dir, "t1", "toolu_A", "sessC")

	stdout, stderr, code := runTP(t, dir, "brief")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"id": "t2"`, "the brief names the next ready task")
	stdout, stderr, code = runTP(t, dir, "next")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"id": "t2"`)
	assert.Equal(t, "toolu_A", claimedBy(t, dir, "t1")["spawn"], "the agent's claim stands")

	_, stderr, code = runTP(t, dir, "unclaim", "t2")
	require.Equal(t, 0, code, stderr)
	spawnClaim(t, dir, "t2", "toolu_B", "sessC")
	stdout, _, code = runTP(t, dir, "next")
	assert.Equal(t, 4, code, stdout)
	assert.Contains(t, stdout, `"agent_held"`, stdout)

	stdout, stderr, code = runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	var res struct {
		Blockers   []any `json:"blockers"`
		NextAction struct {
			Command string `json:"command"`
		} `json:"next_action"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	assert.Empty(t, res.Blockers, stdout)
	assert.Equal(t, "tp pool --session sessC", res.NextAction.Command, stdout)
}

// TestClaimsCountsACloseInAnotherWorktree: the coordinator claims on the main
// tree and the agent closes in its own worktree, so a close or a waiting in
// any worktree's copy accounts for the claim.
func TestClaimsCountsACloseInAnotherWorktree(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, _, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_W1")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "claim", "t2", "--spawn", "toolu_W1")
	require.Equal(t, 0, code)
	unaccounted := func() []any {
		stdout, stderr, code := runTP(t, dir, "claims", "--spawn", "toolu_W1")
		require.Equal(t, 0, code, stderr)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		list, _ := out["unaccounted"].([]any)
		return list
	}
	assert.ElementsMatch(t, []any{"t1", "t2"}, unaccounted())

	worktreeWithDoneCopy(t, dir, "t1")
	_, stderr, code := runTP(t, dir, "set", "t2", "waiting=a long run")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, unaccounted(), "done in the worktree and waiting on main both account")

	stdout, _, code := runTP(t, dir, "claims", "--spawn", "toolu_none")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, `"unaccounted":[]`)
}

// TestClaimsDoesNotCountAStaleHoldInAnotherWorktree: a hold is waiting on an
// open task. A worktree cut while t1 was held keeps that hold after the main
// tree claims t1, and it must not pass for the claimed agent's parking.
func TestClaimsDoesNotCountAStaleHoldInAnotherWorktree(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, stderr, code := runTP(t, dir, "set", "t1", "waiting=the release tag")
	require.Equal(t, 0, code, stderr)
	gitIn(t, dir, "commit", "-q", "-am", "hold")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)

	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_H1")
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code := runTP(t, dir, "claims", "--spawn", "toolu_H1")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"unaccounted":["t1"]`, "the worktree's open, held copy is not the agent parking its claim")
}

// hookRun runs one shipped plugin hook with tp on PATH (or not) and the
// payload on stdin.
func hookRun(t *testing.T, rel string, withTP bool, payload map[string]any, extraEnv ...string) (stdout, stderr string, code int) {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	path := "/usr/bin:/bin"
	if withTP {
		path = filepath.Dir(binaryPath) + ":" + path
	}
	cmd := exec.Command(filepath.Join(repoRootDir(t), filepath.FromSlash(rel))) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = append([]string{"PATH=" + path, "NO_COLOR=1", "TP_HC=0"}, extraEnv...)
	cmd.Dir = t.TempDir()
	cmd.Stdin = bytes.NewReader(raw)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		require.True(t, errors.As(runErr, &exitErr), "the hook must exit rather than fail to start: %v", runErr)
		code = exitErr.ExitCode()
	}
	return out.String(), errb.String(), code
}

func spawnPayload(dir, toolUseID, prompt string) map[string]any {
	return map[string]any{
		"session_id":      "sess-h",
		"cwd":             dir,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Agent",
		"tool_use_id":     toolUseID,
		"tool_input":      map[string]any{"description": "unit", "prompt": prompt},
	}
}

func TestAgentClaimHookClaimsTheMarkedTaskAtSpawn(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)

	_, stderr, code := hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_H1", "Do the unit.\ntp-task: t1 spec.tasks.json\nThen stop."))
	require.Equal(t, 0, code, stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, "wip", task["status"])
	assert.Equal(t, map[string]any{"session": "sess-h", "spawn": "toolu_H1"}, task["claimed_by"])
}

func TestAgentClaimHookPassesAPromptWithoutAMarker(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, stderr, code := hookRun(t, agentClaimHook, false, spawnPayload(dir, "toolu_H2", "Research something, no task here."))
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"])
}

// TestAgentClaimHookReadsOnlyWholeMarkerLinesOfThePrompt: a marker is a line
// of tool_input.prompt that starts with "tp-task: ". A marker quoted inside a
// sentence, or one in any other field, is not a request to claim.
func TestAgentClaimHookReadsOnlyWholeMarkerLinesOfThePrompt(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	inline := spawnPayload(dir, "toolu_M1", "The brief's marker looks like tp-task: t1 spec.tasks.json and is claimed.")
	_, stderr, code := hookRun(t, agentClaimHook, true, inline)
	assert.Equal(t, 0, code, stderr)
	elsewhere := spawnPayload(dir, "toolu_M1", "No marker in this prompt.")
	elsewhere["tool_input"].(map[string]any)["description"] = "tp-task: t1 spec.tasks.json"
	elsewhere["note"] = "x\ntp-task: t1 spec.tasks.json\n"
	_, stderr, code = hookRun(t, agentClaimHook, true, elsewhere)
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"], "neither payload marks a task")

	_, stderr, code = hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_M1", "tp-task: t1 spec.tasks.json"))
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "toolu_M1", claimedBy(t, dir, "t1")["spawn"], "a prompt that is only the marker line claims")
}

func TestAgentClaimHookRefusesTheSpawnWhenTheClaimFails(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	for name, tc := range map[string]struct {
		prompt string
		withTP bool
	}{
		"blocked task":   {"tp-task: t3 spec.tasks.json", true},
		"unknown task":   {"tp-task: nope spec.tasks.json", true},
		"id shape":       {"tp-task: t1$(x) spec.tasks.json", true},
		"path escapes":   {"tp-task: t1 ../spec.tasks.json", true},
		"tp not on PATH": {"tp-task: t1 spec.tasks.json", false},
	} {
		_, stderr, code := hookRun(t, agentClaimHook, tc.withTP, spawnPayload(dir, "toolu_H3", "Unit.\n"+tc.prompt+"\n"))
		assert.Equal(t, 2, code, "%s must refuse the spawn: %s", name, stderr)
		assert.NotEmpty(t, stderr, "%s names its reason", name)
	}
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"])
}

// agentTranscript writes the files Claude Code keeps beside a subagent's
// transcript: the agent's own jsonl and its meta.json, whose toolUseId the
// stop hook reads. The meta shape is the one a probe recorded.
func agentTranscript(t *testing.T, toolUseID string, withMeta bool) string {
	t.Helper()
	sub := filepath.Join(t.TempDir(), "session", "subagents")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	transcript := filepath.Join(sub, "agent-a1b2c3.jsonl")
	require.NoError(t, os.WriteFile(transcript, []byte(`{"type":"user","message":{"content":"x"}}`+"\n"), 0o600))
	if withMeta {
		meta := `{"agentType":"general-purpose","toolUseId":"` + toolUseID + `","finished":true}`
		require.NoError(t, os.WriteFile(strings.TrimSuffix(transcript, ".jsonl")+".meta.json", []byte(meta), 0o600))
	}
	return transcript
}

func stopPayload(dir, transcript string, active bool) map[string]any {
	return map[string]any{
		"session_id":            "sess-h",
		"cwd":                   dir,
		"hook_event_name":       "SubagentStop",
		"stop_hook_active":      active,
		"agent_id":              "a1b2c3",
		"agent_type":            "general-purpose",
		"agent_transcript_path": transcript,
	}
}

func TestSubagentStopHookBlocksAnUnaccountedClaimOnce(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	// toolu_x and the meta.json written by agentTranscript are the fixture
	// a probe recorded: {"agentType":"general-purpose","toolUseId":"toolu_x","finished":true}.
	_, _, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_x")
	require.Equal(t, 0, code)
	transcript := agentTranscript(t, "toolu_x", true)

	stdout, stderr, code := hookRun(t, agentStopHook, true, stopPayload(dir, transcript, false))
	require.Equal(t, 0, code, stderr)
	var decision map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &decision), "the block is the JSON decision: %q", stdout)
	assert.Equal(t, "block", decision["decision"])
	reason, _ := decision["reason"].(string)
	assert.Contains(t, reason, "tp done t1 --file spec.tasks.json", "runnable in a repository with several task files")
	assert.Contains(t, reason, "tp set t1 waiting=")
	assert.Contains(t, reason, "--commit <sha>", "under hc a close needs its commit")

	stdout, _, code = hookRun(t, agentStopHook, true, stopPayload(dir, transcript, true))
	assert.Equal(t, 0, code)
	assert.Empty(t, stdout, "a stop already blocked once passes")

	_, _, code = runTP(t, dir, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code)
	stdout, _, code = hookRun(t, agentStopHook, true, stopPayload(dir, transcript, false))
	assert.Equal(t, 0, code)
	assert.Empty(t, stdout, "a closed claim lets the agent stop")
}

func TestSubagentStopHookPassesWithoutMetaOrClaims(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, _, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_S2")
	require.Equal(t, 0, code)

	stdout, _, code := hookRun(t, agentStopHook, true, stopPayload(dir, agentTranscript(t, "toolu_S2", false), false))
	assert.Equal(t, 0, code)
	assert.Empty(t, stdout, "no meta.json: not a spawn tp can link")

	stdout, _, code = hookRun(t, agentStopHook, true, stopPayload(dir, agentTranscript(t, "toolu_other", true), false))
	assert.Equal(t, 0, code)
	assert.Empty(t, stdout, "a spawn that claimed nothing stops freely")
}

// TestAgentClaimHooksStandAsideInsideATpRunUnit: a tp run unit is the
// driver's to account for, so neither hook claims or blocks there, whatever
// the payload says.
func TestAgentClaimHooksStandAsideInsideATpRunUnit(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, _, code := runTP(t, dir, "claim", "t2", "--spawn", "toolu_U1")
	require.Equal(t, 0, code)
	unit := "TP_UNIT_KIND=implement"

	_, stderr, code := hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_U1", "tp-task: t3 spec.tasks.json"), unit)
	assert.Equal(t, 0, code, "a blocked marker inside a unit is not refused: %s", stderr)
	_, _, code = hookRun(t, agentClaimHook, true, spawnPayload(dir, "toolu_U1", "tp-task: t1 spec.tasks.json"), unit)
	assert.Equal(t, 0, code)
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"], "nothing is claimed inside a unit")

	stdout, _, code := hookRun(t, agentStopHook, true, stopPayload(dir, agentTranscript(t, "toolu_U1", true), false), unit)
	assert.Equal(t, 0, code)
	assert.Empty(t, stdout, "no block inside a unit")
}

func TestAgentClaimHooksAreRegistered(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(repoRootDir(t), "hooks", "hooks.json"))
	require.NoError(t, err)
	var manifest struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))

	found := map[string]string{}
	for event, groups := range manifest.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				found[h.Command] = event + "/" + g.Matcher
			}
		}
	}
	assert.Equal(t, "PreToolUse/Agent", found["${CLAUDE_PLUGIN_ROOT}/"+agentClaimHook])
	assert.Equal(t, "SubagentStop/", found["${CLAUDE_PLUGIN_ROOT}/"+agentStopHook])
	for _, rel := range []string{agentClaimHook, agentStopHook} {
		info, statErr := os.Stat(filepath.Join(repoRootDir(t), filepath.FromSlash(rel)))
		require.NoError(t, statErr, rel)
		assert.NotZero(t, info.Mode().Perm()&0o111, "%s must be executable: a hook without its exec bit fails open", rel)
	}
}

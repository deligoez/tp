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

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
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
	_, _, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A3")
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
		{"one id only", []string{"claim", "t1", "t3", "--spawn", "toolu_B3"}, 2},
		{"session needs spawn", []string{"claim", "t1", "--session", "s"}, 2},
		{"takeover needs spawn", []string{"claim", "t1", "--takeover"}, 2},
	} {
		_, stderr, got := runTP(t, dir, tc.args...)
		assert.Equal(t, tc.code, got, "%s: %s", tc.name, stderr)
	}
	assert.Equal(t, "toolu_A3", claimedBy(t, dir, "t1")["spawn"], "a refused claim leaves the holder")

	_, stderr, _ := runTP(t, dir, "claim", "t1", "--spawn", "toolu_B3")
	assert.Contains(t, stderr, "tp claim t1 --spawn toolu_B3 --takeover", "the refusal names the way out")
	started := taskState(t, dir, "t1")["started_at"]
	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_B3", "--takeover")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "toolu_B3", claimedBy(t, dir, "t1")["spawn"])
	assert.Equal(t, started, taskState(t, dir, "t1")["started_at"], "a takeover keeps the start")
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
	gitIn(t, dir, "commit", "-q", "-am", "claim")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", wt)

	unaccounted := func() []any {
		stdout, stderr, code := runTP(t, dir, "claims", "--spawn", "toolu_W1")
		require.Equal(t, 0, code, stderr)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		list, _ := out["unaccounted"].([]any)
		return list
	}
	assert.ElementsMatch(t, []any{"t1", "t2"}, unaccounted())

	_, stderr, code := runTP(t, wt, "done", "t1", "--gate-passed", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "set", "t2", "waiting=a long run")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, unaccounted(), "done in the worktree and waiting on main both account")

	stdout, _, code := runTP(t, dir, "claims", "--spawn", "toolu_none")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, `"unaccounted":[]`)
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
	_, _, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_S1")
	require.Equal(t, 0, code)
	transcript := agentTranscript(t, "toolu_S1", true)

	stdout, stderr, code := hookRun(t, agentStopHook, true, stopPayload(dir, transcript, false))
	require.Equal(t, 0, code, stderr)
	var decision map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &decision), "the block is the JSON decision: %q", stdout)
	assert.Equal(t, "block", decision["decision"])
	reason, _ := decision["reason"].(string)
	assert.Contains(t, reason, "t1")
	assert.Contains(t, reason, "tp done")
	assert.Contains(t, reason, "waiting=")

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


package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
)

// decodeNow parses a `tp now` payload.
func decodeNow(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "now payload: %s", stdout)
	return out
}

// listTree names every path under dir, so a test can show a read-only command
// left the tree as it found it.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		paths = append(paths, path)
		return err
	}))
	return paths
}

// A project with a .tp/ directory and no task file is still a project: tp now
// exits 0 with task_file and tasks null and every list empty, and writes
// nothing. Outside any project it exits 3, as other commands do.
func TestNow_ProjectWithoutTaskFile(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	project := filepath.Join(dir, "shop")
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".tp"), 0o750))
	before := listTree(t, project)

	stdout, stderr, code := runTP(t, project, "now")
	require.Equal(t, 0, code, "a project with no task file is not an error: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Equal(t, "shop", out["project"])
	assert.Contains(t, out, "task_file")
	assert.Nil(t, out["task_file"])
	assert.Contains(t, out, "tasks")
	assert.Nil(t, out["tasks"])
	assert.Nil(t, out["run"])
	assert.Nil(t, out["epics"])
	for _, key := range []string{"needs_you", "held", "landing", "ready"} {
		assert.Equal(t, []any{}, out[key], "%s is present and empty", key)
	}
	assert.Equal(t, before, listTree(t, project), "tp now writes nothing")

	bare := filepath.Join(dir, "bare")
	require.NoError(t, os.MkdirAll(bare, 0o750))
	_, _, code = runTP(t, bare, "now")
	assert.Equal(t, 3, code, "outside a project tp now exits 3")
}

// A wip task claimed by hand is held by you; tasks counts as tp status does,
// and ready lists the startable tasks in tp ready's order.
func TestNow_PlainWipTaskIsHeldByYou(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	for _, args := range [][]string{
		{"add", `{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Grown.","source_sections":["### 1.1 Seed"],"depends_on":[]}`},
		{"add", `{"id":"trim","title":"Trim","estimate_minutes":5,"acceptance":"Trimmed.","source_sections":["### 1.1 Seed"],"depends_on":["seed"]}`},
		{"claim", "seed"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "%v: %s", args, stderr)
	}

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Equal(t, "spec.tasks.json", out["task_file"])
	assert.Equal(t, "implement", out["phase"])
	assert.Equal(t, map[string]any{"total": 3.0, "done": 0.0, "wip": 1.0, "open": 2.0, "ready": 1.0, "blocked": 1.0, "held": 0.0},
		out["tasks"])
	assert.Equal(t, []any{map[string]any{"id": "grow", "title": "Grow"}}, out["ready"])

	held, ok := out["held"].([]any)
	require.True(t, ok, "held is a list: %v", out["held"])
	require.Len(t, held, 1)
	seed := held[0].(map[string]any)
	assert.Equal(t, "seed", seed["id"])
	assert.Equal(t, "Seed", seed["title"])
	assert.Equal(t, "you", seed["holder"], "a plain wip task with no claim is held by you")
	assert.NotNil(t, seed["since"], "since is the task's started_at")
	assert.Nil(t, seed["worktree"])
	assert.Nil(t, seed["attempt"])
}

// nowHeldOne returns the single held row of a tp now payload.
func nowHeldOne(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	held, ok := out["held"].([]any)
	require.True(t, ok, "held is a list: %v", out["held"])
	require.Len(t, held, 1)
	row, ok := held[0].(map[string]any)
	require.True(t, ok)
	return row
}

// A task a spawn's claim holds is held by an agent.
func TestNow_ClaimedTaskIsHeldByAnAgent(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "claim", "seed", "--spawn", "toolu_now1", "--session", "s1")
	require.Equal(t, 0, code, "claim --spawn: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	row := nowHeldOne(t, decodeNow(t, stdout))
	assert.Equal(t, "seed", row["id"])
	assert.Equal(t, "agent", row["holder"], "a spawn's claim holds the task")
	assert.NotNil(t, row["since"])
}

// A live run's slot: run reports it under running, and the task it runs is
// held by the run, in the run's worktree, at the run's attempt.
func TestNow_LiveRunHoldsItsRunningSlot(t *testing.T) {
	t.Parallel()
	dir, taskFile := liveRunProject(t, true)
	holdLock(t, taskFile)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)

	run, ok := out["run"].(map[string]any)
	require.True(t, ok, "a run state gives a run: %v", out["run"])
	assert.Equal(t, "in_flight", run["run_state"])
	assert.Equal(t, "live", run["run_id"])
	assert.Contains(t, run, "caps")
	assert.Contains(t, run, "last_finished")
	assert.Nil(t, run["last_finished"], "no unit of this run has exited")
	running, ok := run["running"].([]any)
	require.True(t, ok)
	require.Len(t, running, 1)
	slot := running[0].(map[string]any)
	assert.Equal(t, "seed", slot["id"])
	assert.Equal(t, "implement", slot["kind"])

	row := nowHeldOne(t, out)
	assert.Equal(t, "seed", row["id"])
	assert.Equal(t, "run", row["holder"], "a live run holds the task it runs")
	assert.Equal(t, "/elsewhere/seed", row["worktree"])
	assert.Equal(t, 1.0, row["attempt"])
}

// A run driven through the runner seam leaves exited units: last_finished is
// the latest of them, with its exit code and duration.
func TestNow_RunReportsTheLastFinishedUnit(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTPEnv(t, dir, append(seamEnv(t), fakerunner.EnvDurable+"=1"), "run")
	require.Equal(t, 4, code, "the run stops without converging: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	run, ok := decodeNow(t, stdout)["run"].(map[string]any)
	require.True(t, ok, "a run state gives a run")
	assert.Equal(t, "stopped", run["run_state"])
	last, ok := run["last_finished"].(map[string]any)
	require.True(t, ok, "the run's units exited, so one is the last finished: %v", run["last_finished"])
	assert.Equal(t, string(engine.UnitAuditRecord), last["kind"], "the record unit ran last")
	assert.Equal(t, "1", last["id"])
	assert.Equal(t, 2.0, last["attempt"], "its second attempt")
	assert.Contains(t, last, "exit_code")
	assert.NotNil(t, last["duration_seconds"])
}

// A land request made under the task's claim waits for tp land: landing names
// it, and needs_you says so in words that carry no command.
func TestNow_ReportsAWaitingLandRequest(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	spawnClaim(t, dir, "t1", "toolu_L", "sessL")
	require.NoError(t, engine.WriteLandRequest(filepath.Join(dir, "spec.tasks.json"), &engine.LandRequest{
		ID: "t1", Reason: "done", Commits: []string{"0123abcd"}, Worktree: filepath.Join(dir, "wt"), Spawn: "toolu_L",
	}))

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	landing, ok := out["landing"].([]any)
	require.True(t, ok)
	require.Len(t, landing, 1)
	row := landing[0].(map[string]any)
	assert.Equal(t, "t1", row["id"])
	assert.Equal(t, "Task t1", row["title"])
	assert.NotNil(t, row["since"], "since is when the request was made")

	need := needOf(t, out, "land")
	assert.Equal(t, "t1", need["id"])
	assertPlainWords(t, need["text"])
}

// needOf returns the needs_you entry of kind, failing when there is none.
func needOf(t *testing.T, out map[string]any, kind string) map[string]any {
	t.Helper()
	needs, ok := out["needs_you"].([]any)
	require.True(t, ok, "needs_you is a list: %v", out["needs_you"])
	for _, n := range needs {
		if entry, isMap := n.(map[string]any); isMap && entry["kind"] == kind {
			return entry
		}
	}
	require.Failf(t, "no needs_you entry", "kind %q in %v", kind, needs)
	return nil
}

// assertPlainWords checks a needs_you text is a sentence a person reads: no
// tp command and no flag (CLAUDE.md rule 14).
func assertPlainWords(t *testing.T, text any) {
	t.Helper()
	s, ok := text.(string)
	require.True(t, ok, "text is a string: %v", text)
	assert.NotEmpty(t, s)
	assert.NotContains(t, s, "tp ", "no tp command in %q", s)
	assert.NotContains(t, s, "--", "no flag in %q", s)
}

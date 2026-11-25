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

// A project with an epic graph and no task file yet still reports its epics.
func TestNow_EpicsWithoutATaskFile(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitIn(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(epicNowGraph), 0o600))

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Nil(t, out["task_file"])
	assert.Equal(t, []any{
		map[string]any{"id": "payments", "name": "Payment flow", "done": 0.0, "total": 0.0, "state": "open"},
		map[string]any{"id": "later", "name": "Later", "done": 0.0, "total": 0.0, "state": "open"},
	}, out["epics"])
}

// A wip task claimed by hand is held by you; tasks counts as tp status does,
// and ready lists the startable tasks in tp ready's order.
// tp now names the first five ready tasks and counts them all.
func TestNow_NamesTheFirstFiveReadyTasks(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	for _, id := range []string{"t1", "t2", "t3", "t4", "t5", "t6"} {
		task := `{"id":"` + id + `","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["### 1.1 Seed"],"depends_on":[]}`
		_, stderr, code := runTP(t, dir, "add", task)
		require.Equal(t, 0, code, "add %s: %s", id, stderr)
	}
	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Len(t, out["ready"], 5)
	assert.InDelta(t, 7.0, out["tasks"].(map[string]any)["ready"], 0)
}

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

// Under an in-flight run every pool member is the run's, not only the ones
// whose attempt is still running: a member whose unit exited stays pooled
// through integration and the main gate. Its worktree and attempt come from
// its latest row.
func TestNow_LiveRunHoldsEveryPoolMember(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	for _, args := range [][]string{
		{"add", `{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Grown.","source_sections":["### 1.1 Seed"],"depends_on":[]}`},
		{"claim", "seed", "grow"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "%v: %s", args, stderr)
	}
	writeRunState(t, dir, `{"run_id":"pool","started_at":"2026-11-25T00:00:00Z","phase":"implement","stop_reason":null,
 "pool":["seed","grow"],
 "totals":{"units":2,"wall_clock_seconds":9,"spend_usd":0},
 "units":[
  {"seq":1,"kind":"implement","id":"seed","attempt":1,"exit_code":0,"duration_seconds":4,"spend_usd":null,"log_path":"x","worktree":"/wt/seed"},
  {"seq":2,"kind":"implement","id":"grow","attempt":1,"exit_code":null,"duration_seconds":null,"spend_usd":null,"log_path":"y","worktree":"/wt/grow"}]}`)
	holdLock(t, taskFile)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	held, ok := decodeNow(t, stdout)["held"].([]any)
	require.True(t, ok)
	require.Len(t, held, 2)
	byID := map[string]map[string]any{}
	for _, h := range held {
		row := h.(map[string]any)
		byID[row["id"].(string)] = row
	}
	assert.Equal(t, "run", byID["seed"]["holder"], "an exited pool member is still the run's")
	assert.Equal(t, "/wt/seed", byID["seed"]["worktree"])
	assert.Equal(t, 1.0, byID["seed"]["attempt"])
	assert.Equal(t, "run", byID["grow"]["holder"])
	assert.Equal(t, "/wt/grow", byID["grow"]["worktree"])
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

// last_finished is the attempt that finished last, not the one that started
// last: of two parallel units, the earlier-started one that ends later wins.
func TestNow_LastFinishedIsTheLatestToEnd(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	writeRunState(t, dir, `{"run_id":"par","started_at":"2026-11-25T00:00:00Z","phase":"implement","stop_reason":"cap-units",
 "totals":{"units":2,"wall_clock_seconds":10,"spend_usd":0},
 "units":[
  {"seq":1,"kind":"implement","id":"seed","attempt":1,"exit_code":0,"duration_seconds":10,"spend_usd":null,"log_path":"x","started_at":"2026-11-25T00:00:00Z"},
  {"seq":2,"kind":"implement","id":"grow","attempt":1,"exit_code":0,"duration_seconds":3,"spend_usd":null,"log_path":"y","started_at":"2026-11-25T00:00:02Z"}]}`)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	last, ok := decodeNow(t, stdout)["run"].(map[string]any)["last_finished"].(map[string]any)
	require.True(t, ok, "a unit finished")
	assert.Equal(t, "seed", last["id"], "seed ends at 00:00:10, grow at 00:00:05")
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

// A task whose land request waits is in landing, not also in held; tasks
// still counts it as wip.
func TestNow_ALandingTaskIsNotAlsoHeld(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	spawnClaim(t, dir, "t1", "toolu_L", "sessL")
	spawnClaim(t, dir, "t2", "toolu_M", "sessL")
	require.NoError(t, engine.WriteLandRequest(filepath.Join(dir, "spec.tasks.json"), &engine.LandRequest{
		ID: "t1", Reason: "done", Commits: []string{"0123abcd"}, Worktree: filepath.Join(dir, "wt"), Spawn: "toolu_L",
	}))

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	require.Len(t, out["landing"], 1)
	assert.Equal(t, "t2", nowHeldOne(t, out)["id"], "t1 waits to land, so only t2 is held")
	assert.Equal(t, 2.0, out["tasks"].(map[string]any)["wip"], "both tasks are still wip")
}

// A land request record that cannot be read is named on stderr, as tp land
// names it, rather than passed over in silence.
func TestNow_UnreadableLandRequestIsNamed(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	landDir := engine.LandDir(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, os.MkdirAll(landDir, 0o750))
	bad := filepath.Join(landDir, "t1.json")
	require.NoError(t, os.WriteFile(bad, []byte("{not json"), 0o600))

	_, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	assert.Contains(t, stderr, bad, "the unreadable request is named")
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

// A resume blocker reaches needs_you as a plain sentence: here no task is
// ready because the only one is held by something that is not a task.
func TestNow_BlockerBecomesAPlainNeed(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "set", "seed", "waiting=the release tag")
	require.Equal(t, 0, code, "set waiting: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	need := needOf(t, decodeNow(t, stdout), "blocker")
	assertPlainWords(t, need["text"])
	assert.Contains(t, need["text"], "No task is ready")
}

// A run that stopped on an escalation leaves its record: needs_you names the
// decision in plain words, never the record's evidence, which may quote a
// command.
func TestNow_EscalationBecomesADecision(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	writeRunState(t, dir, `{"run_id":"esc","started_at":"2026-11-25T00:00:00Z","phase":"implement","stop_reason":"escalation",
 "totals":{"units":1,"wall_clock_seconds":3,"spend_usd":0},
 "units":[{"seq":1,"kind":"implement","id":"seed","attempt":1,"exit_code":2,"duration_seconds":3,"spend_usd":null,"log_path":"x"}]}`)
	_, err := engine.WriteEscalation(engine.RunDir(dir, "esc"), "1", &engine.Escalation{
		Decision: engine.EscalateRaiseAuditCap, UnitKind: engine.UnitImplement, UnitID: "seed", Phase: "implement",
		Evidence: "raise it with tp set --workflow audit_max_rounds=5", Options: []string{}, At: "2026-11-25T00:00:03Z",
	})
	require.NoError(t, err)
	require.FileExists(t, engine.RunStatePath(dir, taskFile))

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	need := needOf(t, decodeNow(t, stdout), "decision")
	assert.Equal(t, "seed", need["id"], "the unit that escalated")
	assertPlainWords(t, need["text"])
	assert.Contains(t, need["text"], "audit")
}

// Escalation records are read in unit sequence order, numerically: unit 2's
// before unit 10's.
func TestNow_EscalationsInNumericSequenceOrder(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	writeRunState(t, dir, `{"run_id":"esc","started_at":"2026-11-25T00:00:00Z","phase":"implement","stop_reason":"escalation",
 "totals":{"units":2,"wall_clock_seconds":3,"spend_usd":0},
 "units":[{"seq":2,"kind":"implement","id":"early","attempt":1,"exit_code":2,"duration_seconds":3,"spend_usd":null,"log_path":"x"},
          {"seq":10,"kind":"implement","id":"late","attempt":1,"exit_code":2,"duration_seconds":3,"spend_usd":null,"log_path":"y"}]}`)
	for seq, unit := range map[string]string{"2": "early", "10": "late"} {
		_, err := engine.WriteEscalation(engine.RunDir(dir, "esc"), seq, &engine.Escalation{
			Decision: engine.EscalateSkipGate, UnitKind: engine.UnitImplement, UnitID: unit, Phase: "implement",
			Evidence: "e", At: "2026-11-25T00:00:03Z",
		})
		require.NoError(t, err)
	}

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	var ids []any
	for _, n := range decodeNow(t, stdout)["needs_you"].([]any) {
		if need := n.(map[string]any); need["kind"] == "decision" {
			ids = append(ids, need["id"])
		}
	}
	assert.Equal(t, []any{"early", "late"}, ids)
}

// A run whose driver died without a stop reason waits for a person: needs_you
// says so in plain words.
func TestNow_CrashedRunIsANeed(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	writeRunState(t, dir, crashedRunState)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	require.Equal(t, "crashed", out["run"].(map[string]any)["run_state"])
	need := needOf(t, out, "run")
	assertPlainWords(t, need["text"])
}

// A run that stopped on a unit failing every attempt waits for a person:
// needs_you names the unit in plain words.
func TestNow_UnitFailureStopIsANeed(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	writeRunState(t, dir, `{"run_id":"fail","started_at":"2026-11-25T00:00:00Z","phase":"implement","stop_reason":"unit-failure",
 "totals":{"units":1,"wall_clock_seconds":3,"spend_usd":0},
 "units":[{"seq":1,"kind":"implement","id":"seed","attempt":1,"exit_code":1,"duration_seconds":3,"spend_usd":null,"log_path":"x"}]}`)

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	need := needOf(t, decodeNow(t, stdout), "run")
	assert.Equal(t, "seed", need["id"], "the unit that failed")
	assertPlainWords(t, need["text"])
}

// A task file that cannot be parsed may bind no epic at all: it is skipped
// with a notice, and the epics the readable files bind are still reported.
func TestNow_AnUnreadableTaskFileKeepsTheEpics(t *testing.T) {
	t.Parallel()
	dir := epicNowProject(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "other"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other", "broken.tasks.json"), []byte("{not json"), 0o600))

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	epics, ok := decodeNow(t, stdout)["epics"].([]any)
	require.True(t, ok, "one unreadable file does not null the epics: %s", stderr)
	require.Len(t, epics, 2)
	assert.Equal(t, 1.0, epics[0].(map[string]any)["total"], "payments' bound file is still counted")
	assert.Contains(t, stderr, "broken.tasks.json", "the skipped file is named on stderr")
}

// Without an epic graph epics is null; with one it lists each epic by id and
// title, with the done and total tasks of its bound files and whether it is
// done.
func TestNow_EpicsNullWithoutAGraphAndFilledWithOne(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Contains(t, out, "epics")
	assert.Nil(t, out["epics"], "no epic layer, no epics")

	// Bound files are found through git.
	gitIn(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(`{"version":1,"milestones":[["m1"]],
"epics":[
{"id":"payments","title":"Payment flow","milestone":"m1","root":true},
{"id":"later","title":"Later","milestone":"m1","depends_on":[{"id":"payments","why":"w"}]}]}`), 0o600))
	_, stderr, code = runTP(t, dir, "set", "--epic", "payments")
	require.Equal(t, 0, code, "set --epic: %s", stderr)

	stdout, stderr, code = runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	assert.Equal(t, []any{
		map[string]any{"id": "payments", "name": "Payment flow", "done": 0.0, "total": 1.0, "state": "open"},
		map[string]any{"id": "later", "name": "Later", "done": 0.0, "total": 0.0, "state": "open"},
	}, decodeNow(t, stdout)["epics"])
}

// epicNowGraph is a two-epic graph whose root is payments.
const epicNowGraph = `{"version":1,"milestones":[["m1"]],
"epics":[
{"id":"payments","title":"Payment flow","milestone":"m1","root":true},
{"id":"later","title":"Later","milestone":"m1","depends_on":[{"id":"payments","why":"w"}]}]}`

// epicNowProject is runProject in a git repository with epicNowGraph, its
// task file bound to payments.
func epicNowProject(t *testing.T) string {
	t.Helper()
	dir := runProject(t)
	gitIn(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(epicNowGraph), 0o600))
	_, stderr, code := runTP(t, dir, "set", "--epic", "payments")
	require.Equal(t, 0, code, "set --epic: %s", stderr)
	return dir
}

// Several task files and none chosen as active is not an error to the pane:
// tp now exits 0 with no task file, says so in needs_you in plain words, and
// still reports the epics.
func TestNow_SeveralTaskFilesAndNoneActiveIsANeed(t *testing.T) {
	t.Parallel()
	dir := epicNowProject(t)
	body, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.tasks.json"), body, 0o600))

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "several task files is not an error to tp now: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Nil(t, out["task_file"])
	assert.Empty(t, out["phase"])
	assert.Nil(t, out["tasks"])
	need := needOf(t, out, "blocker")
	assert.Equal(t, "no-active-file", need["code"])
	assertPlainWords(t, need["text"])
	epics, ok := out["epics"].([]any)
	require.True(t, ok, "the epics are still read: %v", out["epics"])
	assert.Len(t, epics, 2)
}

// tp now --epic <id> is about that epic: epics lists it alone.
func TestNow_EpicFlagListsThatEpicAlone(t *testing.T) {
	t.Parallel()
	dir := epicNowProject(t)

	stdout, stderr, code := runTP(t, dir, "now", "--epic", "payments")
	require.Equal(t, 0, code, "tp now --epic: %s", stderr)
	assert.Equal(t, []any{
		map[string]any{"id": "payments", "name": "Payment flow", "done": 0.0, "total": 1.0, "state": "open"},
	}, decodeNow(t, stdout)["epics"])
}

// The epics are the task file's project's, not the working directory's: tp
// now --file pointing into another project reports that project's graph.
func TestNow_EpicsComeFromTheTaskFilesProject(t *testing.T) {
	t.Parallel()
	other := epicNowProject(t)
	here := runProject(t)

	stdout, stderr, code := runTP(t, here, "now", "--file", filepath.Join(other, "spec.tasks.json"))
	require.Equal(t, 0, code, "tp now: %s", stderr)
	epics, ok := decodeNow(t, stdout)["epics"].([]any)
	require.True(t, ok, "the other project's graph is read: %s", stderr)
	require.Len(t, epics, 2)
	assert.Equal(t, "payments", epics[0].(map[string]any)["id"])
	assert.Equal(t, 1.0, epics[0].(map[string]any)["total"])
}

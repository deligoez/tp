package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockGraph is spec/1.5.0.md §10 row 3's graph: the chain a→b→c→d with d
// done, and c done too when cDone is set.
func blockGraph(cDone bool) string {
	cRecord := ""
	if cDone {
		cRecord = `, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "e"}`
	}
	return `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "w"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "w"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "w"}]` + cRecord + `},
    {"id": "d", "title": "D", "milestone": "A", "root": true, "done": {"at": "2026-01-02T03:04:05Z", "evidence": "e"}}
  ]
}
`
}

// blockDir returns a project holding blockGraph(false) and spec.tasks.json
// bound to epic, with one task per status given.
func blockDir(t *testing.T, epic string, statuses ...string) string {
	t.Helper()
	dir := epicGraphDir(t, blockGraph(false))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## X\n\nx\n"), 0o600))
	tasks := make([]string, len(statuses))
	for i, s := range statuses {
		tasks[i] = `{"id":"t` + string(rune('1'+i)) + `","title":"T","status":"` + s +
			`","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["## X"]}`
	}
	body := `{"version":1,"spec":"spec.md","epic":"` + epic + `","tasks":[` + strings.Join(tasks, ",") + `]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(body), 0o600))
	return dir
}

// blockedByEpics decodes the blocked_by_epics field of a JSON payload.
func blockedByEpics(t *testing.T, payload string) []string {
	t.Helper()
	var p struct {
		BlockedByEpics []string `json:"blocked_by_epics"`
	}
	require.NoError(t, json.Unmarshal([]byte(payload), &p), "payload: %s", payload)
	return p.BlockedByEpics
}

// taskFileBytes returns spec.tasks.json in dir.
func taskFileBytes(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	return data
}

// TestNext_BlockedByEpicUntilItIsDone is spec/1.5.0.md §10 row 3: while c is
// not done, tp next on b's file reports no task and blocked_by_epics [c],
// claiming nothing; once c is done the next task is offered. A tp next that
// ignored the graph claims t1 and fails.
func TestNext_BlockedByEpicUntilItIsDone(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "open")
	before := taskFileBytes(t, dir)

	stdout, stderr, code := runTP(t, dir, "next")
	require.Equal(t, 4, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Equal(t, []string{"c"}, blockedByEpics(t, stdout))
	assert.NotContains(t, stdout, `"task"`)
	assert.Equal(t, before, taskFileBytes(t, dir), "a blocked tp next claims nothing")

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(blockGraph(true)), 0o600))
	stdout, stderr, code = runTP(t, dir, "next")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"id": "t1"`)
	assert.Empty(t, blockedByEpics(t, stdout))
}

// TestNextPeekAndReady_CarryBlockedByEpics is spec/1.5.0.md §6: tp next
// --peek and tp ready offer no task to start on a blocked file and carry
// blocked_by_epics.
func TestNextPeekAndReady_CarryBlockedByEpics(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "open")

	stdout, stderr, code := runTP(t, dir, "next", "--peek")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Equal(t, []string{"c"}, blockedByEpics(t, stdout))

	stdout, stderr, code = runTP(t, dir, "ready")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, []string{"c"}, blockedByEpics(t, stdout))
	var ready struct {
		Ready []json.RawMessage `json:"ready"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &ready))
	assert.Empty(t, ready.Ready)
}

// TestNextAndReady_ABlockedAnswerTheyCannotWriteExitsThree: a blocked tp next
// or tp ready whose stdout cannot be written is a file error, exit 3, as in tp
// ground (TestOneStdoutWriteErrorIsOneClassification). A blocked answer that dropped the write error
// exits 0 (tp ready) or 4 (tp next) having printed nothing, and fails.
func TestNextAndReady_ABlockedAnswerTheyCannotWriteExitsThree(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "open")

	for _, args := range [][]string{{"next"}, {"next", "--peek"}, {"ready"}, {"ready", "--first"}} {
		stderr, code := runTPUnwritableStdout(t, dir, args...)
		assert.Equal(t, 3, code, "%v stderr: %s", args, stderr)
	}
}

// TestClaim_ExitsOneOnABlockedFile is spec/1.5.0.md §6: tp claim exits 1 on
// a blocked file, names the dependency and writes nothing, by id and with
// --all-ready.
func TestClaim_ExitsOneOnABlockedFile(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "open")
	before := taskFileBytes(t, dir)

	for _, args := range [][]string{{"claim", "t1"}, {"claim", "t1", "t2"}, {"claim", "--all-ready"}} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 1, code, "%v stderr: %s", args, stderr)
		assert.Equal(t, []string{"c"}, blockedByEpics(t, stderr), "%v", args)
		assert.Equal(t, before, taskFileBytes(t, dir), "%v writes nothing", args)
	}
}

// TestNext_ReturnsAWipTaskWhileBlocked is spec/1.5.0.md §6: the block stops
// starting a task, not finishing one, so tp next still returns a wip task.
func TestNext_ReturnsAWipTaskWhileBlocked(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "wip")

	stdout, stderr, code := runTP(t, dir, "next")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"id": "t2"`)
}

// TestNext_AbsentEpicBehavesAsUnboundAndSaysSo is spec/1.5.0.md §6: a file
// bound to an epic the graph does not hold offers its task as an unbound one
// does, with a notice on stderr.
func TestNext_AbsentEpicBehavesAsUnboundAndSaysSo(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "z", "open")

	stdout, stderr, code := runTP(t, dir, "next", "--peek")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"id": "t1"`)
	assert.Contains(t, stderr, `"z"`)
	assert.Contains(t, stderr, "behaves as unbound")
}

// TestDone_RefusesAnOpenTaskOnABlockedFile is spec/1.5.0.md §10 row 3: on a
// blocked file tp done refuses a task that is not already wip — by id it
// exits 1, names the dependency and writes nothing; by several ids and by
// --batch the task is refused and stays open. A tp done that ignored the graph
// claims and closes t1 and fails.
func TestDone_RefusesAnOpenTaskOnABlockedFile(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "open")
	before := taskFileBytes(t, dir)

	_, stderr, code := runTP(t, dir, "done", "t1", "tested by the fixture")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Equal(t, []string{"c"}, blockedByEpics(t, stderr))
	assert.Equal(t, before, taskFileBytes(t, dir), "a refused tp done writes nothing")

	stdout, stderr, code := runTP(t, dir, "done", "t1", "t2", "tested by the fixture")
	require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "epic b is blocked by c")

	batch := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(batch, []byte(`{"id":"t1","reason":"tested by the fixture"}`+"\n"), 0o600))
	stdout, stderr, code = runTP(t, dir, "done", "--batch", batch)
	require.Equal(t, 1, code, "a batch refused by the block exits 1 as by id: stdout: %s stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "epic b is blocked by c")
	var p struct {
		Failures []json.RawMessage `json:"failures"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	require.Len(t, p.Failures, 1, "stdout: %s", stdout)
	assert.Equal(t, []string{"c"}, blockedByEpics(t, string(p.Failures[0])), "the refused row carries the field its hint names")

	assert.NotContains(t, string(taskFileBytes(t, dir)), `"done"`, "no task of the blocked file closed")
}

// TestNextAndReady_AnAllDoneFileAnswersAllDoneWhileBlocked is spec/1.5.0.md
// §6: the block only stops starting a task, so a file whose tasks are all done
// answers as an unbound one does — tp next and tp next --peek report all
// tasks complete, tp ready lists nothing and --first exits 4 with no ready
// tasks — and carries no blocked_by_epics. A block judged before the all-done
// answer reports the blocked epic instead and fails.
func TestNextAndReady_AnAllDoneFileAnswersAllDoneWhileBlocked(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "done", "done")

	for _, args := range [][]string{{"next"}, {"next", "--peek"}} {
		stdout, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 4, code, "%v stdout: %s stderr: %s", args, stdout, stderr)
		var p struct {
			Done    bool   `json:"done"`
			Message string `json:"message"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &p), "%v stdout: %s", args, stdout)
		assert.True(t, p.Done, "%v stdout: %s", args, stdout)
		assert.Equal(t, "All tasks complete", p.Message, "%v", args)
		assert.Empty(t, blockedByEpics(t, stdout), "%v", args)
	}

	stdout, stderr, code := runTP(t, dir, "ready")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.JSONEq(t, "[]", stdout)

	_, stderr, code = runTP(t, dir, "ready", "--first")
	assert.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "no ready tasks")
}

// gateBindsDir returns a project holding blockGraph(false) and an unbound
// spec.tasks.json with two open tasks whose quality gate binds the file to
// the blocked epic b while it runs.
func gateBindsDir(t *testing.T) string {
	t.Helper()
	dir := blockDir(t, "", "open", "open")
	var tf map[string]any
	require.NoError(t, json.Unmarshal(taskFileBytes(t, dir), &tf))
	delete(tf, "epic")
	tf["workflow"] = map[string]any{"quality_gate": binaryPath + " set --epic b"}
	body, err := json.Marshal(tf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), body, 0o600))
	return dir
}

// TestDone_ABindingMadeDuringTheGateRefusesTheOpenTask is spec/1.5.0.md §6:
// the epic block is judged on the file read under the lock, so a gate that
// binds the file to a blocked epic while it runs leaves the open task open —
// by id, by several ids and by --batch. A tp done that judged the file it read
// before the gate closes t1 and fails.
func TestDone_ABindingMadeDuringTheGateRefusesTheOpenTask(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"single", "multi", "batch"} {
		dir := gateBindsDir(t)
		args := []string{"done", "t1", "tested by the fixture"}
		switch mode {
		case "multi":
			args = []string{"done", "t1", "t2", "tested by the fixture"}
		case "batch":
			batch := filepath.Join(dir, "batch.ndjson")
			require.NoError(t, os.WriteFile(batch, []byte(`{"id":"t1","reason":"tested by the fixture"}`+"\n"), 0o600))
			args = []string{"done", "--batch", batch}
		}
		stdout, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 1, code, "%s: stdout: %s stderr: %s", mode, stdout, stderr)
		after := string(taskFileBytes(t, dir))
		require.Contains(t, after, `"epic": "b"`, "%s: the gate bound the file", mode)
		assert.NotContains(t, after, `"status": "done"`, "%s: no open task of the now blocked file closed", mode)
	}
}

// TestDone_ClosesAWipTaskOnABlockedFileAndCountsNoNext is spec/1.5.0.md §6: a
// wip task closes on a blocked file as today, and the next-task counts leave
// the blocked file's open tasks out: ready is 0 and has_next is false.
func TestDone_ClosesAWipTaskOnABlockedFileAndCountsNoNext(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "wip")

	stdout, stderr, code := runTP(t, dir, "done", "t2", "tested by the fixture")
	require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)
	var p struct {
		Closed    string `json:"closed"`
		HasNext   bool   `json:"has_next"`
		Remaining struct {
			Ready int `json:"ready"`
			Open  int `json:"open"`
		} `json:"remaining"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	assert.Equal(t, "t2", p.Closed)
	assert.Equal(t, 1, p.Remaining.Open)
	assert.Equal(t, 0, p.Remaining.Ready, "a blocked file's open task is not ready")
	assert.False(t, p.HasNext)
}

// TestResume_ReportsAnEpicBlockerAndNoImplementUnit is spec/1.5.0.md §10 row
// 3: while c is not done and no task is wip, tp resume on b's file reports the
// block as an escalate blocker naming c and offers no implement unit, so tp
// run stops instead of driving a unit that cannot move. A tp resume that
// ignored the graph offers t1's implement unit and fails.
func TestResume_ReportsAnEpicBlockerAndNoImplementUnit(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "open")

	res := resumeResult(t, dir)
	assert.Equal(t, "implement", res["phase"])
	b := blockerByCode(res, "epic-blocked")
	require.NotNil(t, b, "blockers: %v", res["blockers"])
	assert.Equal(t, "escalate", b["class"])
	data := b["data"].(map[string]any)
	assert.Equal(t, "b", data["epic"])
	assert.Equal(t, []string{"c"}, jsonStrings(data["blocked_by_epics"]))
	assert.Empty(t, res["next_units"], "a blocked file offers no implement unit")
	cmd, payload := nextAction(res)
	assert.Nil(t, cmd)
	assert.Nil(t, payload["task"], "no task to start is previewed")
}

// TestResume_OffersTheWipTaskUnitWhileBlocked is spec/1.5.0.md §10 row 3: with
// a task wip on b's blocked file, tp resume raises no epic blocker and offers
// that task's implement unit as it does today. A tp resume that hid the wip
// task's unit fails.
func TestResume_OffersTheWipTaskUnitWhileBlocked(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open", "wip")

	res := resumeResult(t, dir)
	assert.Nil(t, blockerByCode(res, "epic-blocked"), "blockers: %v", res["blockers"])
	units, _ := res["next_units"].([]any)
	require.Len(t, units, 1, "next_units: %v", res["next_units"])
	unit := units[0].(map[string]any)
	assert.Equal(t, "implement", unit["kind"])
	assert.Equal(t, "t2", unit["id"])
}

// TestResume_AbsentEpicBehavesAsUnboundAndSaysSo is spec/1.5.0.md §6: tp
// resume on a file bound to an epic the graph does not hold, or with no graph
// file, offers the task as on an unbound file and says so on stderr, as tp
// next, tp ready and tp claim do. A tp resume that kept quiet fails.
func TestResume_AbsentEpicBehavesAsUnboundAndSaysSo(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "z", "open")
	noGraph := blockDir(t, "b", "open")
	require.NoError(t, os.Remove(filepath.Join(noGraph, ".tp", "epics.json")))

	for _, d := range []string{dir, noGraph} {
		stdout, stderr, code := runTP(t, d, "resume")
		require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)
		assert.Contains(t, stdout, `"id": "t1"`, "the task's implement unit is offered")
		assert.Contains(t, stderr, "behaves as unbound")
	}
}

// TestDone_RefusesABlockedTaskBeforeJudgingItsEvidence is spec/1.5.0.md §6:
// single-id tp done judges the epic block before the closure evidence, as the
// multi-id and batch paths do, so an agent learns the task cannot close
// before it rewrites its evidence. A tp done that verified the evidence first
// answers with a closure error and fails.
func TestDone_RefusesABlockedTaskBeforeJudgingItsEvidence(t *testing.T) {
	t.Parallel()
	dir := blockDir(t, "b", "open")
	before := taskFileBytes(t, dir)

	_, stderr, code := runTP(t, dir, "done", "t1", "x")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Equal(t, []string{"c"}, blockedByEpics(t, stderr))
	assert.NotContains(t, stderr, "closure verification failed")
	assert.Equal(t, before, taskFileBytes(t, dir), "a refused tp done writes nothing")
}

// corruptDepDir returns blockDir(t, "b", "open", "open") as a real git
// repository holding c/c.tasks.json, a task file that does not parse: it may
// be the one bound to b's dependency c, so nothing can say whether c is done.
func corruptDepDir(t *testing.T) string {
	t.Helper()
	dir := blockDir(t, "b", "open", "open")
	require.NoError(t, os.Remove(filepath.Join(dir, ".git")))
	gitOut(t, dir, "init", "-q", ".")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "c"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c", "c.tasks.json"), []byte("{"), 0o600))
	return dir
}

// TestNextAndEpicStatus_AnUnparsableTaskFileExitsThree is spec/1.5.0.md §6:
// a task file under the walk that cannot be parsed may be bound to any epic,
// so tp next on b's file and tp epic status exit 3 naming it rather than
// guess its epic's status. A walk that skipped it reads c as planned: tp next
// exits 4 blocked and tp epic status exits 0, and both fail.
func TestNextAndEpicStatus_AnUnparsableTaskFileExitsThree(t *testing.T) {
	t.Parallel()
	dir := corruptDepDir(t)
	before := taskFileBytes(t, dir)

	for _, args := range [][]string{{"next"}, {"epic", "status"}} {
		stdout, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 3, code, "%v stdout: %s stderr: %s", args, stdout, stderr)
		assert.Contains(t, stderr, "c.tasks.json", "%v names the file", args)
	}
	assert.Equal(t, before, taskFileBytes(t, dir), "tp next claims nothing")
}

// TestClaim_AnAllDoneFileAnswersAsUnboundWhileBlocked is spec/1.5.0.md §6:
// the block only stops starting a task, so on a file whose tasks are all done
// tp claim --all-ready and tp claim t1 answer exactly as on an unbound file —
// exit 4, no ready task or t1 not open — with no blocked_by_epics. A claim
// that judged the block first exits 1 naming c and fails.
func TestClaim_AnAllDoneFileAnswersAsUnboundWhileBlocked(t *testing.T) {
	t.Parallel()
	blocked := blockDir(t, "b", "done", "done")
	unbound := blockDir(t, "", "done", "done")

	for _, args := range [][]string{{"claim", "--all-ready"}, {"claim", "t1"}} {
		_, wantStderr, wantCode := runTP(t, unbound, args...)
		require.Equal(t, 4, wantCode, "%v on the unbound file: %s", args, wantStderr)
		stdout, stderr, code := runTP(t, blocked, args...)
		assert.Equal(t, wantCode, code, "%v stdout: %s stderr: %s", args, stdout, stderr)
		assert.Equal(t, wantStderr, stderr, "%v", args)
		assert.NotContains(t, stderr, "blocked_by_epics", "%v", args)
	}
}

// TestClaim_AnIdThatIsNotOpenAnswersAsUnboundWhileBlocked is spec/1.5.0.md
// §6, judged per id as tp done judges: on a blocked file that still holds an
// open task, tp claim of a done task or of an id the file does not hold
// answers exactly as on an unbound file — exit 4, not open or not found — with
// no blocked_by_epics. A claim that judged the whole file first exits 1 naming
// c and fails.
func TestClaim_AnIdThatIsNotOpenAnswersAsUnboundWhileBlocked(t *testing.T) {
	t.Parallel()
	blocked := blockDir(t, "b", "open", "done")
	unbound := blockDir(t, "", "open", "done")

	for _, args := range [][]string{{"claim", "t2"}, {"claim", "zz"}, {"claim", "t2", "zz"}} {
		wantStdout, wantStderr, wantCode := runTP(t, unbound, args...)
		require.Equal(t, 4, wantCode, "%v on the unbound file: %s", args, wantStderr)
		stdout, stderr, code := runTP(t, blocked, args...)
		assert.Equal(t, wantCode, code, "%v stdout: %s stderr: %s", args, stdout, stderr)
		assert.Equal(t, wantStderr, stderr, "%v", args)
		assert.Equal(t, wantStdout, stdout, "%v", args)
		assert.NotContains(t, stderr, "blocked_by_epics", "%v", args)
	}
}

// TestCommit_RefusesAnOpenTaskOnABlockedFile is spec/1.5.0.md §6: tp commit
// implicitly claims an open task, so on a blocked file it refuses one that is
// not already wip — exit 1, blocked_by_epics, nothing committed or written —
// while a wip task commits as on an unbound file. A tp commit that ignored the
// graph starts and commits t1 and fails.
func TestCommit_RefusesAnOpenTaskOnABlockedFile(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"open", "wip"} {
		dir := blockDir(t, "b", status)
		require.NoError(t, os.Remove(filepath.Join(dir, ".git")))
		initGitRepo(t, dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o600))
		before := taskFileBytes(t, dir)

		stdout, stderr, code := runTP(t, dir, "commit", "t1", "implemented")
		log := gitOut(t, dir, "log", "--format=%s")
		if status == "wip" {
			require.Equal(t, 0, code, "a wip task commits: stdout: %s stderr: %s", stdout, stderr)
			assert.Contains(t, log, "feat(t1)")
			continue
		}
		require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
		assert.Equal(t, []string{"c"}, blockedByEpics(t, stderr))
		assert.NotContains(t, log, "feat(t1)", "nothing is committed")
		assert.Equal(t, before, taskFileBytes(t, dir), "t1 stays open")
	}
}

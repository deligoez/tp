package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// foundWaitFixture seeds the §4 queue case: t1 is the wip finder (its claim
// is the JSON given, or none), t2 is open, depends on t1 and touches b.go, and
// t3 is an unrelated open task. A blocking request at b.go:1 attaches to t2,
// and the finder's edge onto t2 would close t1 -> t2 -> t1, so apply queues it.
func foundWaitFixture(t *testing.T, claim string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","tasks":[
		{"id":"t1","title":"T1","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]`+claim+`},
		{"id":"t2","title":"T2","status":"open","depends_on":["t1"],"touches":["b.go"],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"t3","title":"T3","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	stdout, stderr, code := foundAdd(t, dir, nil, "--location", "b.go:1", "--reproduction", "go test -run TestB",
		"--expected", "exit 0", "--observed", "exit 1", "--task", "t1", "--blocks")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	return dir
}

// readySet is tp ready's ready set, as ids.
func readySet(t *testing.T, dir string) []string {
	t.Helper()
	stdout, stderr, code := runTPEnv(t, dir, nil, "ready", "--ids")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	var ids []string
	require.NoError(t, json.Unmarshal([]byte(stdout), &ids), stdout)
	return ids
}

// finderStatus reads the fixture finder t1's status from the task file.
func finderStatus(t *testing.T, dir string) string {
	t.Helper()
	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "t1")
	require.NoError(t, err)
	return task.Status
}

// settleQueueEntry moves an open queue entry out of the open state, as a
// rejection by the person does.
func settleQueueEntry(t *testing.T, dir, id string) {
	t.Helper()
	require.NoError(t, engine.UpdateFoundQueue(filepath.Join(dir, "spec.tasks.json"), func(entries []engine.FoundQueueEntry) ([]engine.FoundQueueEntry, error) {
		for i := range entries {
			if entries[i].ID == id {
				entries[i].State = "rejected"
			}
		}
		return entries, nil
	}))
}

// §4: after a blocking request that becomes a queue entry, the finder is open
// and not in tp next's ready set; once the entry is rejected it is ready.
func TestFoundApply_AQueuedFinderWaitsUntilItsEntryIsSettled(t *testing.T) {
	t.Parallel()
	dir := foundWaitFixture(t, "")

	out := foundApply(t, dir)

	require.Equal(t, []any{map[string]any{"request": "F1", "entry": "Q1"}}, out["queued"])
	assert.Equal(t, model.StatusOpen, finderStatus(t, dir), "apply returns the finder to open")
	assert.Equal(t, []string{"t3"}, readySet(t, dir), "the finder waits on its entry")
	stdout, stderr, code := runTPEnv(t, dir, nil, "next", "--peek")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	peeked, ok := parseJSON(t, stdout)["task"].(map[string]any)
	require.True(t, ok, stdout)
	assert.Equal(t, "t3", peeked["id"], "tp next does not offer the finder")

	settleQueueEntry(t, dir, "Q1")

	assert.Equal(t, []string{"t1", "t3"}, readySet(t, dir), "a rejected entry's finder is ready again")
}

// §10 row "An entry holds only its finders": one open entry holding one
// finder, and one unrelated task that stays ready. The mutant that marks
// every open task waiting while an entry is open empties the ready set.
func TestReady_AnEntryHoldsOnlyItsFinders(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","tasks":[
		{"id":"finder","title":"F","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"other","title":"O","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	require.NoError(t, engine.WriteFoundQueue(filepath.Join(dir, "spec.tasks.json"), []engine.FoundQueueEntry{
		{State: engine.FoundQueueOpen, Location: "x.go:1", Question: "Which way?", Finders: []string{"finder"}},
	}))

	assert.Equal(t, []string{"other"}, readySet(t, dir))
}

// Round-4 findings 7 and 11: apply leaves a wip finder under a live spawn
// claim as it is; once the claim is gone (its agent exited without closing),
// the next apply, with no request left to apply, returns it to open.
func TestFoundApply_LeavesAClaimedFinderWipUntilItsClaimIsGone(t *testing.T) {
	t.Parallel()
	dir := foundWaitFixture(t, `,"claimed_by":{"spawn":"toolu_1","token":"tok"}`)

	foundApply(t, dir)
	assert.Equal(t, model.StatusWIP, finderStatus(t, dir), "a live claim keeps its task")

	stdout, stderr, code := runTPEnv(t, dir, nil, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	require.Equal(t, model.StatusWIP, finderStatus(t, dir))

	foundApply(t, dir)
	assert.Equal(t, model.StatusOpen, finderStatus(t, dir))
}

// The same for the unit that filed the request: its own apply leaves its
// task wip, and an apply after it exits returns it to open.
func TestFoundApply_LeavesTheCallingUnitsFinderWip(t *testing.T) {
	t.Parallel()
	dir := foundWaitFixture(t, "")

	stdout, stderr, code := runTPEnv(t, dir, []string{"TP_UNIT_KIND=implement", "TP_UNIT_ID=t1"}, "found", "apply")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Equal(t, model.StatusWIP, finderStatus(t, dir), "the running unit keeps its task")

	foundApply(t, dir)
	assert.Equal(t, model.StatusOpen, finderStatus(t, dir))
}

// And for a main-tree unit of a live tp run: while the run holds its lock and
// the unit's row has no exit code, apply leaves the finder wip.
func TestFoundApply_LeavesALiveRunUnitsFinderWip(t *testing.T) {
	t.Parallel()
	dir := foundWaitFixture(t, "")
	taskFile := filepath.Join(dir, "spec.tasks.json")
	rec, err := engine.NewRunRecorder(dir, taskFile, "run-1", "implement")
	require.NoError(t, err)
	require.NoError(t, rec.StartUnit(&engine.RunUnitRow{Seq: 1, Kind: engine.UnitImplement, ID: "t1", Attempt: 1}))

	require.NoError(t, engine.WithRunLock(taskFile, func() error {
		foundApply(t, dir)
		return nil
	}))
	assert.Equal(t, model.StatusWIP, finderStatus(t, dir), "the run's unit keeps its task")

	foundApply(t, dir)
	assert.Equal(t, model.StatusOpen, finderStatus(t, dir), "with the run gone the unit is gone")
}

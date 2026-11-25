package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// foundHandbackFixture seeds t1, an open finder whose blocking request at
// a.go:1 tp found apply turns into found-f1 (so t1 depends on found-f1), and
// t3, an unrelated open task.
func foundHandbackFixture(t *testing.T) string {
	t.Helper()
	return foundHandbackFixtureWith(t, `{}`)
}

// foundHandbackFixtureWith is foundHandbackFixture with workflow as the task
// file's workflow override.
func foundHandbackFixtureWith(t *testing.T, workflow string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":`+workflow+`,"tasks":[
		{"id":"t1","title":"T1","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"t3","title":"T3","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	stdout, stderr, code := foundAdd(t, dir, nil, "--location", "a.go:1", "--reproduction", "go test -run TestA",
		"--expected", "exit 0", "--observed", "exit 1", "--task", "t1", "--blocks")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	out := foundApply(t, dir)
	require.Equal(t, []any{map[string]any{"request": "F1", "task": "found-f1"}}, out["opened"])
	require.Equal(t, []string{"found-f1"}, taskDeps(t, dir)["t1"])
	require.Equal(t, model.StatusOpen, finderStatus(t, dir), "apply returns the blocked finder to open")
	return dir
}

// §3: after tp found handback the found task is done and marked handed back,
// and the queue holds an open entry with its request.
func TestFoundHandback_ClosesTheTaskAndQueuesItsRequest(t *testing.T) {
	t.Parallel()
	dir := foundHandbackFixture(t)

	stdout, stderr, code := runTPEnv(t, dir, nil, "found", "handback", "found-f1", "TestA passes at HEAD")

	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	out := parseJSON(t, stdout)
	assert.Equal(t, "found-f1", out["handed_back"], stdout)
	assert.Equal(t, "Q1", out["entry"], stdout)
	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, "found-f1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusDone, task.Status)
	assert.True(t, task.HandedBack, "the task is marked handed back")
	require.NotNil(t, task.ClosedReason)
	assert.Contains(t, *task.ClosedReason, "TestA passes at HEAD")
	queue, err := engine.ReadFoundQueue(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, "Q1", queue[0].ID)
	assert.Equal(t, engine.FoundQueueOpen, queue[0].State)
	assert.Equal(t, "F1", queue[0].Request)
	assert.Equal(t, "a.go:1", queue[0].Location)
	assert.Equal(t, "go test -run TestA", queue[0].Reproduction)
	assert.Equal(t, "exit 0", queue[0].Expected)
	assert.Equal(t, "exit 1", queue[0].Observed)
}

// §3/§4: a finder that depended on the handed-back task is a finder of the
// entry and waits on it, out of the ready set, though its dependency is done.
func TestFoundHandback_TheDependentFinderWaitsOnTheEntry(t *testing.T) {
	t.Parallel()
	dir := foundHandbackFixture(t)

	stdout, stderr, code := runTPEnv(t, dir, nil, "found", "handback", "found-f1", "TestA passes at HEAD")

	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Equal(t, []any{"t1"}, parseJSON(t, stdout)["finders"], stdout)
	assert.Equal(t, []string{"t3"}, readySet(t, dir), "t1 waits on the entry")

	settleQueueEntry(t, dir, "Q1")

	assert.Equal(t, []string{"t1", "t3"}, readySet(t, dir), "a settled entry releases its finder")
}

// §3: handback on a task without origin exits 2 and writes nothing.
func TestFoundHandback_ATaskWithoutOriginExitsTwo(t *testing.T) {
	t.Parallel()
	dir := foundHandbackFixture(t)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	stdout, stderr, code := runTPEnv(t, dir, nil, "found", "handback", "t3", "not a found task")

	assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "no origin")
	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the task file is untouched")
	queue, err := engine.ReadFoundQueue(taskFile)
	require.NoError(t, err)
	assert.Empty(t, queue, "no entry is queued")
}

// §12.2: tp found handback holds the task-file lock. A lock held past
// lock_timeout_seconds exits 4 (STATE) with LockTimeoutError's message and
// hint, not the exit 3 of a failed write.
func TestFoundHandback_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	dir := foundHandbackFixtureWith(t, `{"lock_timeout_seconds":1}`)

	release := holdTaskFileLock(t, filepath.Join(dir, "spec.tasks.json"))
	defer release()

	_, stderr, code := runTPEnv(t, dir, nil, "found", "handback", "found-f1", "TestA passes at HEAD")
	assert.Equal(t, 4, code, "a lock held past lock_timeout_seconds exits 4 (STATE): %s", stderr)
	assertLockTimeoutErrorObject(t, stderr)
}

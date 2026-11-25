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

// foundSettleFixture seeds a spec at found_max 1 with three open tasks and a
// queue of two open entries: Q1, a Fix-shaped request past the cap holding
// t1, and Q2, a question holding t2.
func foundSettleFixture(t *testing.T) string {
	t.Helper()
	return foundSettleFixtureWith(t, `{"found_max":1}`)
}

// foundSettleFixtureWith is foundSettleFixture under the given workflow block.
func foundSettleFixtureWith(t *testing.T, workflow string) string {
	t.Helper()
	dir := foundApplyFixture(t, workflow)
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":`+workflow+`,"tasks":[
		{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"t3","title":"T3","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	require.NoError(t, engine.WriteFoundQueue(filepath.Join(dir, "spec.tasks.json"), []engine.FoundQueueEntry{
		{State: engine.FoundQueueOpen, Request: "F1", Location: "x.go:4", Question: "found_max (1) is reached: open a task for this defect?",
			Reproduction: "go test ./x", Expected: "exit 0", Observed: "exit 1", Finders: []string{"t1"}},
		{State: engine.FoundQueueOpen, Request: "F2", Location: "y.go:2", Question: "Which way?", Alternatives: []string{"a", "b"},
			Reproduction: "go test ./y", Finders: []string{"t2"}},
	}))
	return dir
}

func foundSettle(t *testing.T, dir string, flags ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runTPEnv(t, dir, nil, append([]string{"found", "settle"}, flags...)...)
}

// §6: one tp found settle call makes a task of one entry and rejects
// another. The made task does not count against found_max, its entry's
// finder now depends on it, and the rejected entry's finder is ready again.
func TestFoundSettle_MakesATaskOfOneEntryAndRejectsAnother(t *testing.T) {
	t.Parallel()
	dir := foundSettleFixture(t)
	require.Equal(t, []string{"t3"}, readySet(t, dir), "both finders wait on their entries")

	stdout, stderr, code := foundSettle(t, dir, "--task", "Q1", "--reject", "Q2=moved to BUGS.md")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	out := parseJSON(t, stdout)
	assert.Equal(t, []any{map[string]any{"entry": "Q1", "task": "found-q1"}}, out["made"])
	assert.Equal(t, []any{"Q2"}, out["rejected"])

	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	made, _, err := model.FindTask(tf, "found-q1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusOpen, made.Status)
	require.NotNil(t, made.Origin)
	assert.Equal(t, "Q1", made.Origin.QueueEntry)
	assert.Equal(t, "x.go:4", made.Origin.Location)
	assert.Empty(t, made.DependsOn)
	finder, _, err := model.FindTask(tf, "t1")
	require.NoError(t, err)
	assert.Equal(t, []string{"found-q1"}, finder.DependsOn, "the finder waiting on Q1 now depends on its task")

	q := readQueue(t, dir)
	require.Len(t, q, 2)
	assert.Equal(t, engine.FoundQueueSettled, q[0].State)
	assert.Equal(t, "found-q1", q[0].Task)
	assert.Equal(t, engine.FoundQueueRejected, q[1].State)
	assert.Equal(t, "moved to BUGS.md", q[1].Reason)

	assert.ElementsMatch(t, []string{"t2", "t3", "found-q1"}, readySet(t, dir),
		"the rejected entry's finder is ready; the made task's finder waits on it")

	// The cap is 1 and the file holds one task the person made: a new
	// Fix-shaped request still opens a found task.
	fileFixRequest(t, dir, 1)
	applied := foundApply(t, dir)
	assert.Len(t, applied["opened"], 1, "a task the person made does not count against found_max: %v", applied)
	assert.EqualValues(t, 1, applied["found_tasks"])
}

// §4: a queued finder is ready again once tp found settle rejects its entry.
func TestFoundSettle_ARejectedEntrysFinderIsReady(t *testing.T) {
	t.Parallel()
	dir := foundWaitFixture(t, "")
	foundApply(t, dir)
	require.Equal(t, []string{"t3"}, readySet(t, dir))

	stdout, stderr, code := foundSettle(t, dir, "--reject", "Q1=not a defect after all")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)

	assert.Equal(t, []string{"t1", "t3"}, readySet(t, dir))
}

// An entry without the Fix fields takes them from the call; a refused batch
// writes neither the task file nor the queue.
func TestFoundSettle_RefusesABadBatchAndWritesNothing(t *testing.T) {
	t.Parallel()
	for name, flags := range map[string][]string{
		"unknown entry":          {"--task", "Q9"},
		"reject without reason":  {"--reject", "Q2="},
		"entry named twice":      {"--task", "Q1", "--reject", "Q1=no"},
		"question without fix":   {"--task", "Q1", "--task", "Q2"},
		"fix field for a reject": {"--reject", "Q2=no", "--expected", "Q2=a"},
		"nothing to settle":      {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := foundSettleFixture(t)
			taskFile := filepath.Join(dir, "spec.tasks.json")
			before, err := os.ReadFile(taskFile)
			require.NoError(t, err)
			queueBefore, err := os.ReadFile(engine.FoundQueuePath(taskFile))
			require.NoError(t, err)

			stdout, stderr, code := foundSettle(t, dir, flags...)

			assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
			after, err := os.ReadFile(taskFile)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
			queueAfter, err := os.ReadFile(engine.FoundQueuePath(taskFile))
			require.NoError(t, err)
			assert.Equal(t, string(queueBefore), string(queueAfter))
		})
	}
}

// A settled entry cannot be settled again; a question entry made a task
// carries the Fix fields the call gave it.
func TestFoundSettle_AQuestionEntryTakesItsFixFieldsFromTheCall(t *testing.T) {
	t.Parallel()
	dir := foundSettleFixture(t)

	stdout, stderr, code := foundSettle(t, dir, "--task", "Q2", "--expected", "Q2=it takes way a", "--observed", "Q2=it takes way b")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	stdout, stderr, code = runTPEnv(t, dir, nil, "brief", "found-q2")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout, "it takes way a")
	assert.Contains(t, stdout, "go test ./y")

	_, _, code = foundSettle(t, dir, "--reject", "Q2=again")
	assert.Equal(t, 2, code, "a settled entry is not open")
}

// §12.2: a task-file lock held past lock_timeout_seconds makes settle exit 4
// (STATE) with LockTimeoutError's message and hint, as found apply, answer,
// add and handback do, and leaves the queue as it was.
func TestFoundSettle_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	dir := foundSettleFixtureWith(t, `{"found_max":1,"lock_timeout_seconds":1}`)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	queueBefore, err := os.ReadFile(engine.FoundQueuePath(taskFile))
	require.NoError(t, err)

	release := holdTaskFileLock(t, taskFile)
	defer release()

	_, stderr, code := foundSettle(t, dir, "--reject", "Q2=moved to BUGS.md")
	assert.Equal(t, 4, code, "a lock held past lock_timeout_seconds exits 4 (STATE): %s", stderr)
	assertLockTimeoutErrorObject(t, stderr)
	queueAfter, err := os.ReadFile(engine.FoundQueuePath(taskFile))
	require.NoError(t, err)
	assert.Equal(t, string(queueBefore), string(queueAfter), "a timed-out settle writes no queue")
}

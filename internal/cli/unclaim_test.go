package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unclaimPayload is tp unclaim's stdout, the same shape for one id or many.
type unclaimPayload struct {
	Unclaimed   []string `json:"unclaimed"`
	AlreadyOpen []string `json:"already_open"`
	Failed      []struct {
		ID    string `json:"id"`
		Error string `json:"error"`
		Hint  string `json:"hint"`
	} `json:"failed"`
}

func runUnclaim(t *testing.T, dir string, ids ...string) (p unclaimPayload, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"unclaim"}, ids...)...)
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout must be the unclaim payload: %q (stderr %q)", stdout, stderr)
	require.NotNil(t, p.Unclaimed, "unclaimed is [] not null")
	require.NotNil(t, p.AlreadyOpen, "already_open is [] not null")
	require.NotNil(t, p.Failed, "failed is [] not null")
	return p, code
}

// There was no way back from wip: a claimed task could only be closed. tp
// unclaim returns it to open and clears everything claiming set, so the task
// is indistinguishable from one that was never claimed and can be claimed again.
func TestUnclaim_ReturnsAWipTaskToOpen(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	_, _, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code)
	require.Equal(t, "claimed", showTask(t, dir, "t1")["duration_source"], "precondition: claim sets duration_source")

	p, code := runUnclaim(t, dir, "t1")
	require.Equal(t, 0, code)
	assert.Equal(t, []string{"t1"}, p.Unclaimed)
	assert.Empty(t, p.AlreadyOpen)
	assert.Empty(t, p.Failed)

	task := showTask(t, dir, "t1")
	assert.Equal(t, "open", task["status"])
	assert.Nil(t, task["started_at"], "unclaim clears started_at")
	_, hasSource := task["duration_source"]
	assert.False(t, hasSource, "unclaim clears duration_source, which claim set")

	_, _, code = runTP(t, dir, "claim", "t1")
	assert.Equal(t, 0, code, "an unclaimed task can be claimed again")
}

// Batch parity: several ids in one call, each reported in its own bucket, and
// a partial failure does not stop the others from being unclaimed.
func TestUnclaim_BatchReportsEveryID(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	for _, id := range []string{"w1", "w2", "o1", "d1"} {
		addTaskWithEstimate(t, dir, id, 5)
	}
	_, _, code := runTP(t, dir, "claim", "w1", "w2", "d1")
	require.Equal(t, 0, code)
	_, stderr, code := runTP(t, dir, "close", "d1", "d1 acceptance criteria met completely")
	require.Equal(t, 0, code, "close d1: %s", stderr)

	p, code := runUnclaim(t, dir, "w1", "o1", "d1", "missing", "w2")
	require.Equal(t, 0, code, "a partial success exits 0, as tp claim's batch does")
	assert.Equal(t, []string{"w1", "w2"}, p.Unclaimed)
	assert.Equal(t, []string{"o1"}, p.AlreadyOpen, "unclaiming an open task is a reported no-op")
	require.Len(t, p.Failed, 2)
	assert.Equal(t, "d1", p.Failed[0].ID)
	assert.Contains(t, p.Failed[0].Hint, "tp reopen", "a done task is returned to open by tp reopen")
	assert.Equal(t, "missing", p.Failed[1].ID)

	assert.Equal(t, "open", showTask(t, dir, "w1")["status"])
	assert.Equal(t, "open", showTask(t, dir, "w2")["status"])
	assert.Equal(t, "done", showTask(t, dir, "d1")["status"], "a failed id is left untouched")
}

// Unclaiming a done task is a wrong status transition: it fails with the
// state code and a hint naming the command that does move done to open.
func TestUnclaim_DoneTaskFailsWithReopenHint(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	runTP(t, dir, "claim", "t1")
	_, stderr, code := runTP(t, dir, "close", "t1", "t1 acceptance criteria met completely")
	require.Equal(t, 0, code, "close: %s", stderr)

	p, code := runUnclaim(t, dir, "t1")
	assert.Equal(t, 4, code, "a wrong status transition exits 4, as tp claim and tp reopen do")
	assert.Empty(t, p.Unclaimed)
	require.Len(t, p.Failed, 1)
	assert.Contains(t, p.Failed[0].Error, "done")
	assert.Contains(t, p.Failed[0].Hint, "tp reopen t1")
	assert.Equal(t, "done", showTask(t, dir, "t1")["status"])
}

// Unclaiming an open task changes nothing, not even the task file's
// updated_at, and says so rather than failing.
func TestUnclaim_OpenTaskIsANoOp(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	files, err := filepath.Glob(filepath.Join(dir, "*.tasks.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	before, err := os.ReadFile(files[0])
	require.NoError(t, err)

	p, code := runUnclaim(t, dir, "t1")
	require.Equal(t, 0, code)
	assert.Empty(t, p.Unclaimed)
	assert.Equal(t, []string{"t1"}, p.AlreadyOpen)
	assert.Empty(t, p.Failed)

	after, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "a no-op does not rewrite the task file")
}

// `tp remove` told the user to `tp reopen` a wip task, which reopen refuses:
// reopen moves done to open. The hint now names the command that works.
func TestRemove_WipTaskHintsUnclaim(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	runTP(t, dir, "claim", "t1")

	_, stderr, code := runTP(t, dir, "remove", "t1")
	require.Equal(t, 4, code)
	hint, _ := errJSON(t, stderr)["hint"].(string)
	assert.Contains(t, hint, "tp unclaim t1")
	assert.NotContains(t, hint, "reopen", "reopen refuses a wip task")
}

func TestRemove_DoneTaskStillHintsReopen(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "t1 acceptance criteria met completely")

	_, stderr, code := runTP(t, dir, "remove", "t1")
	require.Equal(t, 4, code)
	hint, _ := errJSON(t, stderr)["hint"].(string)
	assert.Contains(t, hint, "tp reopen t1")
}

func TestReopen_WipTaskHintsUnclaim(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	runTP(t, dir, "claim", "t1")

	_, stderr, code := runTP(t, dir, "reopen", "t1")
	require.Equal(t, 4, code)
	hint, _ := errJSON(t, stderr)["hint"].(string)
	assert.Contains(t, hint, "tp unclaim t1")
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupHCRepoWithClosedTask creates a committed hc-strategy project with two
// open tasks (t1, t2), then closes t1 with a recorded sha. After this the task
// file is dirty (t1's closure written but uncommitted, per hc) and the project
// is in the implement phase (t2 still open), so resume reports both bookkeeping
// and the implement-phase guidance note.
func setupHCRepoWithClosedTask(t *testing.T) string {
	t.Helper()
	dir := setupCloseStrategyProject(t, "hc")
	_, _, code := runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 done","source_sections":["s1"]}`)
	require.Equal(t, 0, code)
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 done")
	require.Equal(t, 0, code, "hc closes t1 with --commit: %s", stderr)
	return dir
}

// bookkeepingByPath indexes resume's bookkeeping array by path.
func bookkeepingByPath(res map[string]any) map[string]map[string]any {
	arr, _ := res["bookkeeping"].([]any)
	out := make(map[string]map[string]any, len(arr))
	for _, e := range arr {
		m := e.(map[string]any)
		out[m["path"].(string)] = m
	}
	return out
}

// TestResume_Bookkeeping_ClosureForClosedTaskFile covers §5.2: under hc, a
// modified task file (closed task) appears in bookkeeping with kind=closure and
// ref=task id, NOT in changes, and raises no unexplained-changes blocker.
func TestResume_Bookkeeping_ClosureForClosedTaskFile(t *testing.T) {
	dir := setupHCRepoWithClosedTask(t)

	res := resumeResult(t, dir)

	changes := jsonStrings(res["changes"])
	assert.NotContains(t, changes, "spec.tasks.json", "the dirty task file is not an unexplained change")

	bk := bookkeepingByPath(res)
	entry, ok := bk["spec.tasks.json"]
	require.True(t, ok, "the dirty task file appears in bookkeeping")
	assert.Equal(t, "closure", entry["kind"])
	assert.Equal(t, "t1", entry["ref"], "closure ref is the task whose managed fields changed")

	assert.Nil(t, blockerByCode(res, "unexplained-changes"),
		"a tp-owned task file raises no unexplained-changes blocker")
}

// TestResume_Bookkeeping_RoundFile covers §5.2: a dirty .tp-review/ round file
// is bookkeeping with kind=round and ref=round number parsed from the filename.
func TestResume_Bookkeeping_RoundFile(t *testing.T) {
	dir := setupHCRepoWithClosedTask(t)
	reviewDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(reviewDir, 0o755))
	// A valid state.json keeps the .tp-review/ dir from reading as corrupt.
	require.NoError(t, os.WriteFile(filepath.Join(reviewDir, "state.json"),
		[]byte(`{"spec":"spec.md","review_rounds":[],"audit_rounds":[]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(reviewDir, "review-round-1.ndjson"), []byte("{}\n"), 0o600))

	res := resumeResult(t, dir)
	bk := bookkeepingByPath(res)
	entry, ok := bk[".tp-review/spec/review-round-1.ndjson"]
	require.True(t, ok, "the .tp-review round file appears in bookkeeping")
	assert.Equal(t, "round", entry["kind"])
	assert.Equal(t, "1", entry["ref"], "round ref is the round number parsed from the filename")
}

// TestResume_Bookkeeping_ConfigFile covers §5.2: a dirty .tp/ state file is
// bookkeeping with kind=config and ref=path basename.
func TestResume_Bookkeeping_ConfigFile(t *testing.T) {
	dir := setupHCRepoWithClosedTask(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"), []byte("{}\n"), 0o600))

	res := resumeResult(t, dir)
	bk := bookkeepingByPath(res)
	entry, ok := bk[".tp/config.json"]
	require.True(t, ok, "a dirty .tp/ state file appears in bookkeeping")
	assert.Equal(t, "config", entry["kind"])
	assert.Equal(t, "config.json", entry["ref"], "config ref is the path basename")
}

// TestResume_Bookkeeping_EmptyWhenClean covers §5.2: bookkeeping is [] when no
// tp-owned file is dirty.
func TestResume_Bookkeeping_EmptyWhenClean(t *testing.T) {
	dir := newResumeRepo(t) // committed, clean working tree
	res := resumeResult(t, dir)
	assert.Equal(t, []any{}, res["bookkeeping"], "bookkeeping is [] when no tp-owned file is dirty")
}

// TestResume_Guidance_PresentAtImplement covers the execution-model guidance
// note: it is emitted when phase is implement.
func TestResume_Guidance_PresentAtImplement(t *testing.T) {
	dir := setupHCRepoWithClosedTask(t) // t2 open -> implement phase
	res := resumeResult(t, dir)
	assert.Equal(t, "implement", res["phase"])
	guidance, ok := res["guidance"]
	require.True(t, ok, "guidance is emitted at the implement phase")
	assert.Contains(t, guidance.(string), "fresh subagent")
}

// TestResume_Guidance_AbsentOutsideImplement covers the guidance note's absence
// outside the implement phase.
func TestResume_Guidance_AbsentOutsideImplement(t *testing.T) {
	// Zero tasks, no review state -> review phase; no guidance.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[]}`), 0o600))
	initGitRepo(t, dir)

	res := resumeResult(t, dir)
	assert.Equal(t, "review", res["phase"])
	_, ok := res["guidance"]
	assert.False(t, ok, "guidance is omitted outside the implement phase")
}

// TestResume_BookkeepingAndGuidance_SurviveCompact covers §8.4: both bookkeeping
// and the guidance note survive --compact.
func TestResume_BookkeepingAndGuidance_SurviveCompact(t *testing.T) {
	dir := setupHCRepoWithClosedTask(t) // implement phase + dirty task file

	out, stderr, code := runTP(t, dir, "resume", "--compact")
	require.Equal(t, 0, code, "resume --compact: %s", stderr)
	var compact map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &compact))

	require.Contains(t, compact, "bookkeeping", "bookkeeping survives --compact")
	bk := bookkeepingByPath(compact)
	assert.Equal(t, "closure", bk["spec.tasks.json"]["kind"], "closure entry survives --compact")
	require.Contains(t, compact, "guidance", "guidance survives --compact")
	assert.Contains(t, compact["guidance"].(string), "fresh subagent")
}

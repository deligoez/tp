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


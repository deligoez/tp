package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// heldProject is a file with two ready tasks, the second held by a gate that
// is not a task: waiting set on an open task.
func heldProject(t *testing.T) string {
	t.Helper()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "free", 5)
	addTaskWithEstimate(t, dir, "gated", 1)
	_, stderr, code := runTP(t, dir, "set", "gated", "waiting=the release tag")
	require.Equal(t, 0, code, "an open task takes waiting: %s", stderr)
	return dir
}

func TestHeld_ReadySkipsAHeldTask(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	stdout, _, code := runTP(t, dir, "ready", "--ids")
	require.Equal(t, 0, code)
	var ids []string
	require.NoError(t, json.Unmarshal([]byte(stdout), &ids))
	assert.Equal(t, []string{"free"}, ids, "a held task is not ready")
}

func TestHeld_NextSkipsAHeldTask(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	stdout, _, code := runTP(t, dir, "next", "--peek")
	require.Equal(t, 0, code)
	got := parseJSON(t, stdout)["task"].(map[string]any)
	assert.Equal(t, "free", got["id"], "tp next picks past a held task, though it has the smaller estimate")
}

func TestHeld_ClaimAllReadySkipsAHeldTask(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	_, _, code := runTP(t, dir, "claim", "--all-ready")
	require.Equal(t, 0, code)
	assert.Equal(t, "open", showTask(t, dir, "gated")["status"])
	assert.Equal(t, "wip", showTask(t, dir, "free")["status"])
}

func TestHeld_AClaimClearsWaiting(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	_, _, code := runTP(t, dir, "claim", "gated")
	require.Equal(t, 0, code, "an explicit claim takes a held task")
	task := showTask(t, dir, "gated")
	assert.Equal(t, "wip", task["status"])
	assert.Nil(t, task["waiting"], "the hold ends with the claim")
}

func TestHeld_UnclaimClearsWaiting(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "w", 5)
	_, _, code := runTP(t, dir, "claim", "w")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "w", "waiting=a long run")
	require.Equal(t, 0, code)
	_, code = runUnclaim(t, dir, "w")
	require.Equal(t, 0, code)
	assert.Nil(t, showTask(t, dir, "w")["waiting"], "a task back at open is not silently held")
}

func TestHeld_DoneTaskStillRefusesWaiting(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "d", 5)
	_, _, code := runTP(t, dir, "done", "d", "d acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "d", "waiting=x")
	assert.NotEqual(t, 0, code, "a done task waits on nothing")
}

func TestHeld_ExportCarriesHeldState(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	stdout, stderr, code := runTP(t, dir, "export")
	require.Equal(t, 0, code, stderr)
	var exp struct {
		Tasks []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &exp))
	states := map[string]string{}
	for _, tk := range exp.Tasks {
		states[tk.ID] = tk.State
	}
	assert.Equal(t, map[string]string{"free": "ready", "gated": "held"}, states)
}

func TestHeld_StatusCountsAHeldTaskApartFromReady(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	stdout, _, code := runTP(t, dir, "status")
	require.Equal(t, 0, code)
	got := parseJSON(t, stdout)
	assert.InDelta(t, 1, got["ready"], 0, "only the free task is ready")
	assert.InDelta(t, 1, got["held"], 0, "the held task is counted as held")
}

func TestHeld_ResumeNamesHeldTasksWhenNothingElseIsReady(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "gated", 1)
	_, _, code := runTP(t, dir, "set", "gated", "waiting=the release tag")
	require.Equal(t, 0, code)
	stdout, _, _ := runTP(t, dir, "resume")
	assert.Contains(t, stdout, "gated", "resume names the held task")
	assert.Contains(t, stdout, "the release tag", "and what holds it")
}

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// unitIDs lists the unit ids the fake runner recorded, in spawn order.
func unitIDs(t *testing.T, records string) []string {
	t.Helper()
	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	ids := make([]string, 0, len(recs))
	for i := range recs {
		ids = append(ids, recs[i].Env[engine.EnvUnitID])
	}
	return ids
}

// runTask reads one task of the project's task file.
func runTask(t *testing.T, dir, id string) model.Task {
	t.Helper()
	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	task, _, err := model.FindTask(tf, id)
	require.NoError(t, err)
	return *task
}

// unitAttempts returns the attempt number of every run-state row of id.
func unitAttempts(t *testing.T, dir, id string) []int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "run-spec.json"))
	require.NoError(t, err)
	var st struct {
		Units []struct {
			ID      string `json:"id"`
			Attempt int    `json:"attempt"`
		} `json:"units"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	var attempts []int
	for _, u := range st.Units {
		if u.ID == id {
			attempts = append(attempts, u.Attempt)
		}
	}
	return attempts
}

// spec/1.31.0.md §2: tp run applies after every unit, so a request an
// implement unit files is a found task in the file when the run ends, and
// the run goes on to implement it.
func TestRunCommand_AnImplementUnitsRequestEndsAsAFoundTask(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	env, records := seam(t, fakerunner.EnvDurable+"=1", fakerunner.EnvFoundIDs+"=seed")
	_, stderr, _ := runTPEnv(t, dir, env, "run")

	assert.Equal(t, []string{"found-f1"}, foundTasksIn(t, dir), stderr)
	found := runTask(t, dir, "found-f1")
	assert.Equal(t, []string{"seed"}, found.DependsOn, "a non-blocking found task depends on its finder")
	ids := unitIDs(t, records)
	require.GreaterOrEqual(t, len(ids), 2, stderr)
	assert.Equal(t, []string{"seed", "found-f1"}, ids[:2], "the found task is the run's next unit")
	reqs, err := engine.ReadFoundRequests(dir, filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, engine.FoundStateApplied, reqs[0].State)
}

// §10 row "A blocking placement spends no attempt": a unit that files a
// blocking request and exits without closing is not a failed attempt. With
// no retries left the mutant that counts the exit stops the run with
// unit-failure after one unit; tp run instead applies the request, runs the
// found task, then runs the finder again as attempt 1.
func TestRunCommand_ABlockingPlacementSpendsNoAttempt(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "run_max_unit_retries=0")
	require.Equal(t, 0, code, stderr)
	env, records := seam(t, fakerunner.EnvDurable+"=1", fakerunner.EnvBlockIDs+"=seed")
	_, stderr, _ = runTPEnv(t, dir, env, "run")

	ids := unitIDs(t, records)
	require.GreaterOrEqual(t, len(ids), 3, stderr)
	assert.Equal(t, []string{"seed", "found-f1", "seed"}, ids[:3], stderr)
	seed := runTask(t, dir, "seed")
	assert.Contains(t, seed.DependsOn, "found-f1", "the finder depends on its blocking request's found task")
	assert.Equal(t, model.StatusDone, seed.Status)
	assert.Equal(t, []int{1, 1}, unitAttempts(t, dir, "seed"), "the finder's next run is attempt 1 again")
}

// The same row in a rolling pool: the finder's slot is not retried, the
// found task runs, and the finder runs again in a worktree of its own.
func TestRunCommand_ABlockingPlacementSpendsNoAttemptInAPool(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	for _, set := range [][]string{
		{"set", "seed", `touches=["seed.txt"]`}, {"set", "grow", `touches=["grow.txt"]`},
		{"set", "--workflow", "run_max_parallel=2"}, {"set", "--workflow", "run_max_unit_retries=0"},
	} {
		_, stderr, code = runTP(t, dir, set...)
		require.Equal(t, 0, code, "%v: %s", set, stderr)
	}
	initGitRepo(t, dir)
	// grow outlasts the found task, so the finder runs again inside the same
	// pool; its rerun needs the found task's commit (found-f1.txt) in its
	// tree, whichever tree it runs in.
	env, records := seam(t, fakerunner.EnvDurable+"=1", fakerunner.EnvBlockIDs+"=seed",
		fakerunner.EnvSleepByID+"=grow=3000", fakerunner.EnvRequireByID+"=seed=found-f1.txt")
	_, stderr, _ = runTPEnv(t, dir, env, "run")

	ids := unitIDs(t, records)
	seedRuns := 0
	for _, id := range ids {
		if id == "seed" {
			seedRuns++
		}
	}
	assert.Equal(t, 2, seedRuns, "%v: %s", ids, stderr)
	assert.True(t, slices.Contains(ids, "found-f1"), "%v: %s", ids, stderr)
	seed := runTask(t, dir, "seed")
	assert.Contains(t, seed.DependsOn, "found-f1")
	assert.Equal(t, model.StatusDone, seed.Status, stderr)
	assert.Equal(t, []int{1, 1}, unitAttempts(t, dir, "seed"))
}

// spec/1.31.0.md §2: an audit unit's requests are applied only after its
// round is recorded, so tp found apply skips a request filed from an audit
// round not yet recorded and applies it once that round is.
func TestFoundApply_SkipsTheRequestsOfAnUnrecordedAuditRound(t *testing.T) {
	t.Parallel()
	dir := foundApplyFixture(t, `{}`)
	taskFile := filepath.Join(dir, "spec.tasks.json")
	_, err := engine.AddFoundRequest(dir, taskFile, &engine.FoundRequest{
		Location: "internal/a.go:4", Reproduction: "go test ./internal", Expected: "exit 0", Observed: "exit 1",
		Finder: engine.FoundFinder{AuditRound: 1, RowKey: "go-safety:file-internal/a.go"},
	})
	require.NoError(t, err)

	held := foundApply(t, dir)
	assert.Nil(t, held["opened"], "the audit round is not recorded yet")
	reqs, err := engine.ReadFoundRequests(dir, taskFile)
	require.NoError(t, err)
	require.Len(t, reqs, 1, "a held request stays in the store")
	assert.Equal(t, engine.FoundStateUnapplied, reqs[0].State)

	recordCleanAuditRounds(t, filepath.Join(dir, "spec.md"), 1)
	applied := foundApply(t, dir)
	assert.Equal(t, []any{map[string]any{"request": "F1", "task": "found-f1"}}, applied["opened"])
}

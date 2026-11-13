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

// TestRunDryRun_SaysABatchRunsInFreshCheckouts: a dry run that would spawn an
// implement batch says once that its worktrees lack ignored files (§3).
func TestRunDryRun_SaysABatchRunsInFreshCheckouts(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	_, stderr, code = runTP(t, dir, "run", "--dry-run")
	require.Equal(t, 0, code, stderr)
	assert.NotContains(t, stderr, "fresh worktree", "no batch at run_max_parallel 1")

	for _, set := range [][]string{
		{"set", "seed", `touches=["seed.txt"]`}, {"set", "grow", `touches=["grow.txt"]`},
		{"set", "--workflow", "run_max_parallel=2"},
	} {
		_, stderr, code = runTP(t, dir, set...)
		require.Equal(t, 0, code, "%v: %s", set, stderr)
	}
	stdout, stderr, code := runTP(t, dir, "run", "--dry-run")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"brief_command": "tp brief grow"`)
	assert.Contains(t, stderr, "fresh worktree of HEAD, without ignored files")
}

// TestRunCommand_DrivesAnImplementBatch: under run_max_parallel 2, tp run
// runs two ready tasks with disjoint touches side by side in worktrees, then
// closes each through tp done on the main branch with the picked sha
// (spec/backlog/parallel-implement.md §2.3, §2.4).
func TestRunCommand_DrivesAnImplementBatch(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "add",
		`{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	for _, set := range [][]string{
		{"set", "seed", `touches=["seed.txt"]`}, {"set", "grow", `touches=["grow.txt"]`},
		{"set", "--workflow", "run_max_parallel=2"},
	} {
		_, stderr, code = runTP(t, dir, set...)
		require.Equal(t, 0, code, "%v: %s", set, stderr)
	}
	initGitRepo(t, dir)

	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))
	_, stderr, _ = runTPEnv(t, dir, []string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
		fakerunner.EnvDurable + "=1",
		fakerunner.EnvSleepMS + "=300",
	}, "run")

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(recs), 2, stderr)
	assert.ElementsMatch(t, []string{"grow", "seed"}, []string{recs[0].Env[engine.EnvUnitID], recs[1].Env[engine.EnvUnitID]})
	assert.True(t, recs[0].SpawnedAt.Before(recs[1].ExitedAt) && recs[1].SpawnedAt.Before(recs[0].ExitedAt),
		"the batch's units run side by side")
	assert.True(t, filepath.IsAbs(recs[0].Env[engine.EnvTaskFile]), "a batched unit is handed the main tree's task file")

	for _, id := range []string{"seed", "grow"} {
		showOut, showErr, showCode := runTP(t, dir, "show", id)
		require.Equal(t, 0, showCode, showErr)
		var shown map[string]any
		require.NoError(t, json.Unmarshal([]byte(showOut), &shown))
		task, _ := shown["task"].(map[string]any)
		if task == nil {
			task = shown
		}
		assert.Equal(t, "done", task["status"], "%s closes through tp done: %s", id, stderr)
		shas, _ := task["commit_shas"].([]any)
		require.Len(t, shas, 1, id)
		assert.Equal(t, "unit "+id, gitRun(t, dir, "log", "-1", "--format=%s", shas[0].(string)))
	}
	assert.FileExists(t, filepath.Join(dir, "seed.txt"))
	assert.FileExists(t, filepath.Join(dir, "grow.txt"))
}

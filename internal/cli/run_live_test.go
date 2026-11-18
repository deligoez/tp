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

// liveRunProject is runProject with three disjoint tasks under
// run_max_parallel 2, seed claimed, and a run state whose one open row is
// seed's: a run that started seed, in a worktree when worktree is non-empty,
// and has not stopped.
func liveRunProject(t *testing.T, worktree string) (dir, taskFile string) {
	t.Helper()
	dir = runProject(t)
	taskFile = filepath.Join(dir, "spec.tasks.json")
	for _, args := range [][]string{
		{"add", `{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Grown.","source_sections":["### 1.1 Seed"],"depends_on":[]}`},
		{"add", `{"id":"trim","title":"Trim","estimate_minutes":5,"acceptance":"Trimmed.","source_sections":["### 1.1 Seed"],"depends_on":[]}`},
		{"set", "seed", `touches=["seed.txt"]`}, {"set", "grow", `touches=["grow.txt"]`}, {"set", "trim", `touches=["trim.txt"]`},
		{"set", "--workflow", "run_max_parallel=2"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "%v: %s", args, stderr)
	}
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	for i := range tf.Tasks {
		if tf.Tasks[i].ID == "seed" {
			tf.Tasks[i].Status = model.StatusWIP
		}
	}
	require.NoError(t, model.WriteTaskFile(taskFile, tf))
	st := engine.RunState{RunID: "live", Phase: engine.PhaseImplement, Units: []engine.RunUnitRow{
		{Seq: 1, Kind: engine.UnitImplement, ID: "seed", Attempt: 1, LogPath: "x", Worktree: worktree},
	}}
	raw, err := json.Marshal(st)
	require.NoError(t, err)
	path := engine.RunStatePath(dir, taskFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return dir, taskFile
}

// holdLock holds the cycle's run lock until the test ends, as a live run does.
func holdLock(t *testing.T, taskFile string) {
	t.Helper()
	acquired, release, held := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(held)
		assert.NoError(t, engine.WithRunLock(taskFile, func() error {
			close(acquired)
			<-release
			return nil
		}))
	}()
	<-acquired
	t.Cleanup(func() {
		close(release)
		<-held
	})
}

func decodeResume(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "resume payload: %s", stdout)
	return out
}

// TestResume_NamesALivePoolAndFillsOnlyItsFreeSlots: while a run holds the
// lock and its pool runs seed in a worktree, tp resume names seed in
// running_units and offers only the one free slot, to grow; it does not offer
// seed again as a wip task to resume (spec/backlog/rolling-implement.md §2).
func TestResume_NamesALivePoolAndFillsOnlyItsFreeSlots(t *testing.T) {
	t.Parallel()
	dir, taskFile := liveRunProject(t, "/elsewhere/seed")
	holdLock(t, taskFile)

	stdout, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	out := decodeResume(t, stdout)
	assert.Equal(t, []any{"seed"}, out["running_units"])
	assert.Equal(t, []string{"implement grow"}, unitPairs(t, out["next_units"]))
}

// TestResume_ADeadRunsOpenRowsAreAbandoned: the same run state with nobody
// holding the lock is a run that died; seed is the wip task to resume alone.
func TestResume_ADeadRunsOpenRowsAreAbandoned(t *testing.T) {
	t.Parallel()
	dir, _ := liveRunProject(t, "/elsewhere/seed")

	stdout, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	out := decodeResume(t, stdout)
	assert.NotContains(t, out, "running_units")
	assert.Equal(t, []string{"implement seed"}, unitPairs(t, out["next_units"]))
}

// TestResume_ACapOneRunsPayloadIsUnchanged: a live run whose open row runs in
// the main tree, as every unit at run_max_parallel 1 does, leaves tp resume's
// payload exactly what it is with no run at all.
func TestResume_ACapOneRunsPayloadIsUnchanged(t *testing.T) {
	t.Parallel()
	dir, taskFile := liveRunProject(t, "")
	before, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)

	holdLock(t, taskFile)
	during, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	assert.JSONEq(t, before, during)
}

// TestRunDryRun_SaysALivePoolIsInProgress: tp run --dry-run over a cycle whose
// pool is running lists no unit to start, names the running ones, and says a
// run is in progress.
func TestRunDryRun_SaysALivePoolIsInProgress(t *testing.T) {
	t.Parallel()
	dir, taskFile := liveRunProject(t, "/elsewhere/seed")
	holdLock(t, taskFile)

	stdout, stderr, code := runTP(t, dir, "run", "--dry-run")
	require.Equal(t, 0, code, stderr)
	out := decodeDryRun(t, stdout)
	assert.Empty(t, unitPairs(t, out["next_units"]))
	assert.Equal(t, []any{"seed"}, out["running_units"])
	assert.Contains(t, stderr, "a tp run is in progress")
}

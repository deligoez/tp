package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// driverWorkflow is the workflow a loop test runs under: caps high enough that
// only the test's own arrangement ends the run, so a loop that fails to
// terminate hangs the test rather than being quietly stopped by a cap.
func driverWorkflow() *model.Workflow {
	return &model.Workflow{RunMaxUnits: 20, RunMaxWallClockSeconds: 600}
}

// seamProject wires a temp project to the fake runner: it builds the fake,
// pins it as TP_RUNNER_SEAM, and points its record directory at a fresh
// directory the test reads invocations back from.
func seamProject(t *testing.T, tasksJSON string) (root, spec, taskFile, records string) {
	t.Helper()
	root, spec, taskFile = setupResumeProject(t, tasksJSON)
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records = filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))
	t.Setenv(EnvRunnerSeam, bin)
	t.Setenv(fakerunner.EnvDir, records)
	return root, spec, taskFile, records
}

// driveOnce runs the loop and fails the test if it does not terminate. A loop
// is the one shape of code whose most informative failure is not a wrong
// answer but no answer at all, so every test here is bounded.
func driveOnce(t *testing.T, root, spec, taskFile string, wf *model.Workflow) DriverResult {
	t.Helper()
	type outcome struct {
		res DriverResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := RunDriver(&DriverOptions{Root: root, TaskFile: taskFile, Spec: spec, Workflow: *wf})
		done <- outcome{res, err}
	}()
	select {
	case got := <-done:
		require.NoError(t, got.err)
		return got.res
	case <-time.After(90 * time.Second):
		t.Fatal("the driver loop did not terminate")
		return DriverResult{}
	}
}

// invocations reads what the fake runner recorded, in spawn order.
func invocations(t *testing.T, records string) []fakerunner.Invocation {
	t.Helper()
	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	return recs
}

// overlapped reports whether two children were alive at the same moment,
// which is how §10.1 test 5 is asserted: from the children's own clocks, not
// from the driver's bookkeeping about itself.
func overlapped(a, b *fakerunner.Invocation) bool {
	return a.SpawnedAt.Before(b.ExitedAt) && b.SpawnedAt.Before(a.ExitedAt)
}

// spawnedIDs lists the TP_UNIT_ID of every recorded invocation, in order.
func spawnedIDs(recs []fakerunner.Invocation) []string {
	ids := make([]string, 0, len(recs))
	for i := range recs {
		ids = append(ids, recs[i].Env[EnvUnitID])
	}
	return ids
}

// twoOpenTasks is a task file holding two independent open tasks, so the
// implement phase yields two units in sequence.
const twoOpenTasks = `{"spec":"s.md","tasks":[
 {"id":"alpha","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]},
 {"id":"beta","title":"B","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`

const oneDoneTask = `{"spec":"s.md","tasks":[{"id":"t","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`

// A releasable cycle is the loop's exit condition and its very first check
// (§3.1 step 2): nothing is spawned, and the reason is converged.
func TestRunDriver_StopsConvergedWhenTheOracleReportsRelease(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneDoneTask)
	recordRounds(t, spec, 0, 2, true)

	res := driveOnce(t, root, spec, taskFile, driverWorkflow())

	assert.Equal(t, StopConverged, res.StopReason)
	assert.Equal(t, PhaseRelease, res.Phase)
	assert.Empty(t, invocations(t, records), "a releasable cycle spawns nothing")
	assert.NotEmpty(t, res.RunID, "every run carries its own id")
}


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

// §3.1 end to end for the implement phase: the loop spawns the unit the oracle
// named, re-reads the cycle from disk, sees the phase advance as the durable
// writes land, and stops with converged. The two implement units never overlap,
// which is §3.3's "Alone" enforced by the driver rather than assumed.
func TestRunDriver_AdvancesThroughImplementAndStopsConverged(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, twoOpenTasks)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvDurable, "1")

	res := driveOnce(t, root, spec, taskFile, driverWorkflow())

	assert.Equal(t, StopConverged, res.StopReason)
	assert.Equal(t, PhaseRelease, res.Phase)

	recs := invocations(t, records)
	require.Len(t, recs, 2, "one implement unit per open task, then release")
	assert.Equal(t, []string{"alpha", "beta"}, spawnedIDs(recs), "the driver spawns the unit the oracle named")
	for i := range recs {
		assert.Equal(t, string(UnitImplement), recs[i].Env[EnvUnitKind])
	}
	assert.False(t, overlapped(&recs[0], &recs[1]), "a non-concurrent kind is never spawned beside a sibling")

	// The run state carries a row per attempt with the child's exit code.
	var st RunState
	data, err := os.ReadFile(RunStatePath(root, taskFile))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &st))
	require.Len(t, st.Units, 2)
	require.NotNil(t, st.Units[0].ExitCode)
	assert.Equal(t, 0, *st.Units[0].ExitCode)
	require.NotNil(t, st.StopReason)
	assert.Equal(t, StopConverged, *st.StopReason)
}

// The two role kinds are the only ones §3.3 lets run together, and the loop
// spawns the whole panel in one iteration. The role files land at their final
// names, which is the driver's rename of the .part each role wrote (§3.3.1);
// the next iteration then has nothing pending and stops with no-units.
func TestRunDriver_RoleUnitsRunConcurrently(t *testing.T) {
	cases := []struct {
		name  string
		tasks string
		phase string
		roles []string
	}{
		{"review", `{"spec":"s.md","tasks":[]}`, PhaseReview, []string{"implementer", "tester", "architect"}},
		{"audit", oneDoneTask, PhaseAudit, []string{"spec-coverage", "security", "maintainability-conventions"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, spec, taskFile, records := seamProject(t, tc.tasks)
			t.Setenv(fakerunner.EnvDurable, "1")
			t.Setenv(fakerunner.EnvSleepMS, "150")

			res := driveOnce(t, root, spec, taskFile, driverWorkflow())

			assert.Equal(t, StopNoUnits, res.StopReason,
				"a collected round owes no further role unit today")
			assert.Equal(t, tc.phase, res.Phase)

			recs := invocations(t, records)
			require.Len(t, recs, len(tc.roles), "one unit per active role, once")
			assert.ElementsMatch(t, tc.roles, spawnedIDs(recs))
			for i := range recs {
				for j := i + 1; j < len(recs); j++ {
					assert.True(t, overlapped(&recs[i], &recs[j]),
						"role siblings run concurrently: %s and %s did not overlap",
						recs[i].Env[EnvUnitID], recs[j].Env[EnvUnitID])
				}
			}

			roundDir := RoundDir(root, taskFile, tc.phase, 1)
			for _, role := range tc.roles {
				assert.FileExists(t, RoleFindingsPath(roundDir, role),
					"the driver renames the role's .part to the final name on exit 0")
			}
		})
	}
}

// §3.1.1: the driver deletes a role's findings file — the final name and any
// stale .part — immediately before spawning that role, so a leftover from an
// earlier attempt can never answer for an attempt that wrote nothing.
func TestRunDriver_RoleLeftoverIsClearedBeforeTheAttempt(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, `{"spec":"s.md","tasks":[]}`)

	// A .part a crashed earlier attempt left behind. The oracle still returns
	// the role — only the final name satisfies its predicate — so the driver
	// is the one that has to clear it, and a driver that did not would rename
	// the leftover into the final name and count a unit that wrote nothing.
	roundDir := RoundDir(root, taskFile, PhaseReview, 1)
	require.NoError(t, os.MkdirAll(roundDir, 0o750))
	leftover := RoleFindingsPath(roundDir, "implementer")
	require.NoError(t, os.WriteFile(leftover+".part", []byte(`{"role":"implementer"}`+"\n"), 0o600))

	// The fake exits 0 and writes nothing this time.
	res := driveOnce(t, root, spec, taskFile, driverWorkflow())

	assert.Equal(t, StopUnitFailure, res.StopReason,
		"an attempt that wrote nothing is a failed attempt")
	assert.NotEmpty(t, invocations(t, records), "the role was re-run rather than skipped")
	assert.NoFileExists(t, leftover,
		"a previous attempt's leftover must never be promoted into this attempt's durable write")
}


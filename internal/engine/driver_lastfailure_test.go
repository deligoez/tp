package engine

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// The cycle these tests drive is driver_retry_test.go's oneOpenTask: the
// smallest one that owes an implement unit, so a scripted exit code lands on
// exactly one unit and nothing else moves.

// §4.2: the driver writes last_failure when a child exits non-zero, with the
// failing command and the log path as the summary — a summary tp authored, so
// the record can never carry the child's own prose into the next unit.
func TestRunDriver_NonZeroExitWritesLastFailure(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)
	t.Setenv(fakerunner.EnvExits, "3")

	res := driveOnce(t, root, spec, taskFile, driverWorkflow())
	require.Equal(t, StopUnitFailure, res.StopReason)
	recs := invocations(t, records)
	require.Len(t, recs, 1)

	got := ReadLastFailure(root, taskFile)
	require.NotNil(t, got, "a non-zero unit exit records last_failure")
	assert.Equal(t, UnitImplement, got.UnitKind)
	assert.Equal(t, "alpha", got.UnitID)
	assert.Equal(t, PhaseImplement, got.Phase)
	assert.Equal(t, 3, got.ExitCode)
	assert.NotEmpty(t, got.At)

	logPath := unitLogPath(RunDir(root, res.RunID), 1, UnitImplement, "alpha")
	assert.Contains(t, got.Summary, logPath, "the summary names the log the failing attempt wrote")
	assert.Contains(t, got.Summary, recs[0].Argv[0],
		"the summary names the command that failed, from the driver's own resolution")
}

// §4.2's trigger is a non-zero exit. An attempt that exits 0 having written
// nothing is a failed attempt under §3.3, but it is not the condition this
// record reports, and inventing one would put an exit code of 0 in a field
// whose whole point is the code the child exited with.
func TestRunDriver_ExitZeroWithNoDurableWriteRecordsNoLastFailure(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)

	res := driveOnce(t, root, spec, taskFile, driverWorkflow())
	require.Equal(t, StopUnitFailure, res.StopReason, "exit 0 with no durable write is still a failed attempt")
	require.Len(t, invocations(t, records), 1)

	assert.Nil(t, ReadLastFailure(root, taskFile))
}

// §4.2: a success of the same unit clears the record. The retry is the shortest
// path through both writers in one run — attempt 1 exits non-zero and records,
// attempt 2 exits 0 with its durable write and clears.
func TestRunDriver_SuccessOfTheSameUnitClearsLastFailure(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvExits, "1")
	t.Setenv(fakerunner.EnvDurable, "0,1")

	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1
	res := driveOnce(t, root, spec, taskFile, wf)

	require.Equal(t, StopConverged, res.StopReason, "the retry finished the unit and the cycle became releasable")
	require.Len(t, invocations(t, records), 2, "one failed attempt and one that succeeded")
	assert.Nil(t, ReadLastFailure(root, taskFile),
		"the next success of that unit clears the record it left")
}

// §4.2: the record lives outside the run state because it must survive the run
// that wrote it. A second run neither clears it at start nor loses it, so the
// fresh process that reads it learns what the previous attempt walked into.
func TestLastFailure_SurvivesIntoTheNextRun(t *testing.T) {
	root, spec, taskFile, _ := seamProject(t, oneOpenTask)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvExits, "2")

	first := driveOnce(t, root, spec, taskFile, driverWorkflow())
	require.Equal(t, StopUnitFailure, first.StopReason)
	before := ReadLastFailure(root, taskFile)
	require.NotNil(t, before)

	// The cycle becomes releasable without that unit ever succeeding, so the
	// second run spawns nothing and touches no unit's record.
	done := strings.ReplaceAll(oneOpenTask, `"status":"open"`, `"status":"done"`)
	require.NoError(t, os.WriteFile(taskFile, []byte(done), 0o600))
	second := driveOnce(t, root, spec, taskFile, driverWorkflow())
	require.Equal(t, StopConverged, second.StopReason)

	after := ReadLastFailure(root, taskFile)
	require.NotNil(t, after, "a new run does not clear a record it did not succeed past")
	assert.Equal(t, before.UnitID, after.UnitID)
	assert.Equal(t, before.At, after.At, "the surviving record is the same object, not a rewritten one")
}

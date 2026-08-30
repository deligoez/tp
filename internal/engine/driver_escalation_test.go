package engine

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// §5.2: a unit that wrote a valid escalation record stops the run with
// escalation, is not counted as a failed attempt — so it is never retried —
// and every artifact is left in place for the operator who has to decide.
func TestRunDriver_ValidEscalationRecordStopsTheRun(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)
	t.Setenv(fakerunner.EnvExits, "2")
	t.Setenv(fakerunner.EnvEscalate, "1")

	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1
	res := driveOnce(t, root, spec, taskFile, wf)

	assert.Equal(t, StopEscalation, res.StopReason)
	require.Len(t, invocations(t, records), 1,
		"an escalating unit is not a failed attempt, so the retry budget is never spent on it")

	path := EscalationPath(RunDir(root, res.RunID), "1")
	assert.FileExists(t, path, "tp run leaves every artifact in place")
	assert.Equal(t, path, res.EscalationPath, "the run reports the record it stopped on")

	st := readRunStateFile(t, root, taskFile)
	require.NotNil(t, st.StopReason)
	assert.Equal(t, StopEscalation, *st.StopReason, "the run state records the same reason")
	require.Len(t, st.Units, 1, "the one attempt is still recorded, with the code its child exited with")
	require.NotNil(t, st.Units[0].ExitCode)
	assert.Equal(t, 2, *st.Units[0].ExitCode)
}

// Test 47: the record, not the exit code, is the signal — the driver spawns a
// harness rather than tp, so the harness's exit code need not carry the inner
// command's.
//
// Every row is an exit code that means something else without the record: 0
// with nothing written is a failed attempt (§3.3), 0 with the durable write is
// a success the loop would carry on past, and a non-zero code is the ordinary
// failure. All three stop the run as an escalation.
func TestRunDriver_EscalatesWhateverTheHarnessExitedWith(t *testing.T) {
	cases := []struct {
		name    string
		exits   string
		durable string
	}{
		{name: "exit 0 with nothing written", exits: "0"},
		{name: "exit 0 with the durable write", exits: "0", durable: "1"},
		{name: "the exit 2 tp escalate itself returns", exits: "2"},
		{name: "an exit code the harness invented", exits: "9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, spec, taskFile, records := seamProject(t, oneOpenTask)
			t.Setenv(fakerunner.EnvExits, tc.exits)
			t.Setenv(fakerunner.EnvDurable, tc.durable)
			t.Setenv(fakerunner.EnvEscalate, "1")

			res := driveOnce(t, root, spec, taskFile, driverWorkflow())

			assert.Equal(t, StopEscalation, res.StopReason)
			assert.Len(t, invocations(t, records), 1, "the run stops at the unit that asked")
			assert.Equal(t, EscalationPath(RunDir(root, res.RunID), "1"), res.EscalationPath)
		})
	}
}

// §5.2: an escalation is a question rather than a failure, so it sets no
// last_failure — and, being no success either, it clears none. The pre-existing
// record is this unit's own, which is the only record a success of it would
// have cleared.
func TestRunDriver_EscalationNeitherWritesNorClearsLastFailure(t *testing.T) {
	root, spec, taskFile, _ := seamProject(t, oneOpenTask)
	require.NoError(t, WriteLastFailure(root, taskFile, &LastFailure{
		UnitKind: UnitImplement, UnitID: "alpha", Phase: PhaseImplement,
		ExitCode: 1, Summary: "an earlier run's failure", At: "2026-08-30T08:00:00Z",
	}))
	t.Setenv(fakerunner.EnvExits, "2")
	t.Setenv(fakerunner.EnvEscalate, "1")

	res := driveOnce(t, root, spec, taskFile, driverWorkflow())
	require.Equal(t, StopEscalation, res.StopReason)

	got := ReadLastFailure(root, taskFile)
	require.NotNil(t, got, "an escalation neither writes a record nor clears the one already there")
	assert.Equal(t, "an earlier run's failure", got.Summary,
		"the record is the one the earlier run left, not one this attempt wrote")
}

// §5.2: a record that fails schema validation is judged by the unit's §3.3
// predicate and its exit code as usual — so the unit is retried and the run
// ends with unit-failure, exactly as if the file were not there.
func TestRunDriver_InvalidEscalationRecordIsAnOrdinaryAttempt(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)
	t.Setenv(fakerunner.EnvExits, "2")
	t.Setenv(fakerunner.EnvEscalate, "bad")

	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1
	res := driveOnce(t, root, spec, taskFile, wf)

	assert.Equal(t, StopUnitFailure, res.StopReason)
	assert.Empty(t, res.EscalationPath)
	assert.Len(t, invocations(t, records), 2, "an ordinary failed attempt spends the retry budget")

	// The file is on disk, so the driver read it and rejected it rather than
	// never having looked.
	assert.FileExists(t, EscalationPath(RunDir(root, res.RunID), "1"))

	got := ReadLastFailure(root, taskFile)
	require.NotNil(t, got, "an ordinary non-zero exit still records last_failure")
	assert.Equal(t, 2, got.ExitCode)
}


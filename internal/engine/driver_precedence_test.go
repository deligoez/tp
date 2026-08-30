package engine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// Section 3.4's precedence rule, driven through the real loop: when one
// checkpoint satisfies several rows the recorded reason is the first of
// converged, driver-error, escalation, unit-failure, interrupted, cap-budget,
// cap-wall-clock, cap-units, no-units.
//
// Every test in this file satisfies two of those conditions at the same
// checkpoint and asserts which one was recorded, because that is the only
// arrangement that can discriminate: a run that trips one condition alone
// records it under any ordering at all. Each pair is therefore driven twice —
// once with both conditions true, and once as a control that trips only the
// losing one and records it, which is what proves the loser was genuinely
// armed in the first arm rather than never reached.

// closesTheOnlyTask is the arrangement both cap arms below share: one open
// task, one implement unit that closes it, and a cycle whose audit rounds
// decide whether what that unit leaves behind is releasable.
//
// auditRounds of 2 makes the cycle releasable the moment the task closes, so
// the checkpoint after that single iteration satisfies converged and the cap at
// once; 1 leaves the cycle in audit, so only the cap is satisfied.
func closesTheOnlyTask(t *testing.T, auditRounds int, sleepMS string, arm func(*model.Workflow)) (DriverResult, RunState, *model.Workflow) {
	t.Helper()
	root, spec, taskFile, _ := seamProject(t, oneOpenTask)
	recordRounds(t, spec, 0, auditRounds, true)
	t.Setenv(fakerunner.EnvDurable, "1")
	if sleepMS != "" {
		t.Setenv(fakerunner.EnvSleepMS, sleepMS)
	}

	wf := driverWorkflow()
	arm(wf)
	res := driveOnce(t, root, spec, taskFile, wf)
	return res, readRunStateFile(t, root, taskFile), wf
}

// converged leads the order, so a run that reached a cap on the very iteration
// that made the cycle releasable records converged rather than the cap: a cycle
// that became releasable is releasable whatever else the iteration also hit.
//
// Both counters are driven, because what is under test is one ordering rather
// than one cap's special case. The losing condition is asserted from the run
// state the driver left: capStop over those totals is the same question the
// loop asked at the checkpoint, so a non-empty answer is the cap having been
// reached at the moment converged won.
func TestRunDriver_ConvergedOutranksACapTheSameIterationReached(t *testing.T) {
	cases := []struct {
		name    string
		capStop StopReason
		sleepMS string
		arm     func(*model.Workflow)
	}{
		{
			name:    "the unit cap",
			capStop: StopCapUnits,
			arm:     func(wf *model.Workflow) { wf.RunMaxUnits = 1 },
		},
		{
			name:    "the wall-clock cap",
			capStop: StopCapWallClock,
			sleepMS: "1200",
			arm:     func(wf *model.Workflow) { wf.RunMaxWallClockSeconds = 1 },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("armed: the cap alone stops a cycle that is not releasable", func(t *testing.T) {
				res, _, _ := closesTheOnlyTask(t, 1, tc.sleepMS, tc.arm)
				assert.Equal(t, tc.capStop, res.StopReason)
				assert.NotEqual(t, PhaseRelease, res.Phase,
					"the control's cycle is one clean audit round short of releasable")
			})

			t.Run("converged wins when the same iteration also released the cycle", func(t *testing.T) {
				res, st, wf := closesTheOnlyTask(t, 2, tc.sleepMS, tc.arm)

				require.Equal(t, tc.capStop, capStop(&st, wf),
					"the cap was reached at the checkpoint, so both rows were satisfied at once")
				assert.Equal(t, StopConverged, res.StopReason,
					"converged leads section 3.4's precedence order, so the cap it tied with is not what the run records")
				assert.Equal(t, PhaseRelease, res.Phase)
			})
		})
	}
}

// Section 5.2 names this pair outright: a unit that wrote a valid escalation
// record stops the run with escalation, subject only to section 3.4's
// precedence, where a cycle that became releasable in the same iteration still
// records converged.
//
// The record on disk is what proves the escalation row was satisfied — the
// driver read it and ranked it, rather than never having seen it — and
// EscalationPath stays empty because the run did not stop on it.
func TestRunDriver_ConvergedOutranksAnEscalationTheSameIterationRaised(t *testing.T) {
	drive := func(t *testing.T, auditRounds int) (DriverResult, string) {
		t.Helper()
		root, spec, taskFile, _ := seamProject(t, oneOpenTask)
		recordRounds(t, spec, 0, auditRounds, true)
		t.Setenv(fakerunner.EnvDurable, "1")
		t.Setenv(fakerunner.EnvEscalate, "1")

		res := driveOnce(t, root, spec, taskFile, driverWorkflow())
		return res, EscalationPath(RunDir(root, res.RunID), "1")
	}

	t.Run("armed: the escalation alone stops a cycle that is not releasable", func(t *testing.T) {
		res, record := drive(t, 1)
		assert.Equal(t, StopEscalation, res.StopReason)
		assert.Equal(t, record, res.EscalationPath)
	})

	t.Run("converged wins when the same iteration also released the cycle", func(t *testing.T) {
		res, record := drive(t, 2)

		require.FileExists(t, record,
			"the unit did write its escalation record, so both rows were satisfied at once")
		assert.Equal(t, StopConverged, res.StopReason,
			"section 5.2: an escalation raised in an iteration that released the cycle still records converged")
		assert.Equal(t, PhaseRelease, res.Phase)
		assert.Empty(t, res.EscalationPath, "the run reports a record only when it stopped on one")
	})
}

// unit-failure outranks every cap: a unit that spent its last attempt on the
// iteration that also reached the unit cap is reported as the failure it is,
// not as a run that was merely bounded.
func TestRunDriver_UnitFailureOutranksACapTheSameIterationReached(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)

	// The fake exits 0 and writes nothing, which is a failed attempt
	// (section 3.3), and no retry is budgeted — so the unit exhausts its
	// attempts on the same iteration that spends the run's only unit.
	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 0
	wf.RunMaxUnits = 1
	res := driveOnce(t, root, spec, taskFile, wf)

	st := readRunStateFile(t, root, taskFile)
	require.Equal(t, StopCapUnits, capStop(&st, wf),
		"the unit cap was reached at the checkpoint, so both rows were satisfied at once")
	assert.Equal(t, StopUnitFailure, res.StopReason,
		"unit-failure precedes cap-units, and a cap must not report an exhausted unit as a bounded run")
	assert.Len(t, invocations(t, records), 1, "the one attempt is all the budget bought")
}

// A cap outranks no-units: a role panel that reached the unit cap and left the
// oracle with nothing pending records the cap, because a run that was cut short
// is not a phase that has run out of work.
func TestRunDriver_ACapOutranksAnEmptyNextUnits(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, `{"spec":"s.md","tasks":[]}`)
	t.Setenv(fakerunner.EnvDurable, "1")

	// Round 1 already carries its recorded entry, so the collected panel owes
	// no record unit either (§4.1) and next_units genuinely empties once the
	// three roles have written their findings.
	recordRounds(t, spec, 0, 0, true) // state.json, which a round file may not exist without
	require.NoError(t, os.WriteFile(roundFilePath(spec, PhaseReview, 1), []byte("\n"), 0o600))

	wf := driverWorkflow()
	wf.RunMaxUnits = 3
	res := driveOnce(t, root, spec, taskFile, wf)

	require.Len(t, invocations(t, records), 3, "the whole review panel ran and wrote its findings")
	_, _, units, err := DryRunUnits(&DriverOptions{Root: root, TaskFile: taskFile, Spec: spec, Workflow: *wf})
	require.NoError(t, err)
	require.Empty(t, units,
		"the oracle owes no further unit, so no-units was satisfied at the same checkpoint")

	st := readRunStateFile(t, root, taskFile)
	require.Equal(t, StopCapUnits, capStop(&st, wf), "and so was the unit cap")
	assert.Equal(t, StopCapUnits, res.StopReason,
		"cap-units precedes no-units: the run was bounded, not out of work")
}

package engine

import (
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


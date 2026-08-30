package engine

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// §3.4's table, transcribed. A run stops for exactly one of these reasons and
// records it verbatim, so the values are pinned here rather than derived: a
// constant that drifts from the spec's spelling is a stop nobody can match on.
func TestStopReason_IsSection34sNineValuesVerbatim(t *testing.T) {
	cases := []struct {
		reason StopReason
		want   string
	}{
		{StopConverged, "converged"},
		{StopCapUnits, "cap-units"},
		{StopCapWallClock, "cap-wall-clock"},
		{StopCapBudget, "cap-budget"},
		{StopEscalation, "escalation"},
		{StopUnitFailure, "unit-failure"},
		{StopNoUnits, "no-units"},
		{StopInterrupted, "interrupted"},
		{StopDriverError, "driver-error"},
	}
	require.Len(t, cases, 9, "§3.4 names nine stop reasons")

	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		assert.Equal(t, tc.want, string(tc.reason))
		assert.True(t, tc.reason.Known(), "%s is one of the nine", tc.want)
		assert.False(t, seen[tc.want], "%s is declared once", tc.want)
		seen[tc.want] = true
	}
}

// The vocabulary is closed: anything that is not one of the nine — including
// the empty string capStop returns while a run is within every cap — is not a
// stop reason.
func TestStopReason_KnownRejectsEverythingElse(t *testing.T) {
	for _, near := range []StopReason{"", "cap_units", "CONVERGED", "capunits", "cap-unit", "done", "converged "} {
		assert.False(t, near.Known(), "%q is not one of §3.4's nine", string(near))
	}
}

// The guard sits at the sink: the recorder is the one writer of stop_reason,
// so a reason outside the vocabulary is refused there rather than trusted from
// whichever caller composed it, and the file keeps whatever it already held.
func TestRunRecorder_StopRefusesAReasonOutsideTheVocabulary(t *testing.T) {
	root := t.TempDir()
	rec, err := NewRunRecorder(root, "s.tasks.json", "run", PhaseImplement)
	require.NoError(t, err)

	require.NoError(t, rec.Stop(StopCapUnits))
	require.Error(t, rec.Stop("cap_units"), "an unrecognized reason is never recorded")

	st, err := ReadRunState(root, "s.tasks.json")
	require.NoError(t, err)
	require.NotNil(t, st.StopReason)
	assert.Equal(t, StopCapUnits, *st.StopReason, "the refused write left the recorded reason alone")
}

// Each cap is produced by its own counter and by no other. The negative rows
// are the load-bearing ones: a check reading the wrong total still stops a run,
// and only a case where the other two counters are far from their caps can tell
// the difference.
func TestCapStop_EachCapIsProducedByItsOwnCounter(t *testing.T) {
	wf := func(units, seconds int, budget float64) *model.Workflow {
		return &model.Workflow{RunMaxUnits: units, RunMaxWallClockSeconds: seconds, RunMaxBudgetUSD: budget}
	}
	st := func(units, seconds int, spend float64) *RunState {
		return &RunState{Totals: RunTotals{Units: units, WallClockSeconds: seconds, SpendUSD: spend}}
	}
	caps := wf(10, 100, 5)

	cases := []struct {
		name  string
		state *RunState
		wf    *model.Workflow
		want  StopReason
	}{
		{"within every cap", st(9, 99, 4.99), caps, ""},
		{"units reached, nothing else near", st(10, 1, 0), caps, StopCapUnits},
		{"wall clock reached, nothing else near", st(1, 100, 0), caps, StopCapWallClock},
		{"budget reached, nothing else near", st(1, 1, 5), caps, StopCapBudget},
		{"units one below", st(9, 1, 0), caps, ""},
		{"wall clock one below", st(1, 99, 0), caps, ""},
		{"budget just below", st(1, 1, 4.99), caps, ""},
		{"units past its cap", st(11, 1, 0), caps, StopCapUnits},
		{"budget zero is disabled, not lowest", st(1, 1, 999), wf(10, 100, 0), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, capStop(tc.state, tc.wf))
		})
	}
}

// cap-budget accrues through the recorder rather than through a total the
// driver keeps beside it, so the cap is exercised over the rows a metered
// runner produced. An unmetered row contributes nothing and leaves the cap
// inert, which is the same path §3.2.2 describes.
func TestCapStop_BudgetAccruesFromTheMeteredRows(t *testing.T) {
	root := t.TempDir()
	rec, err := NewRunRecorder(root, "s.tasks.json", "run", PhaseImplement)
	require.NoError(t, err)

	spend := 0.75
	for seq, amount := range []*float64{&spend, nil, &spend} {
		require.NoError(t, rec.StartUnit(&RunUnitRow{Seq: seq, Kind: UnitImplement, ID: "t", Attempt: 1}))
		require.NoError(t, rec.FinishUnit(seq, 0, time.Second, amount))
	}

	snapshot := rec.Snapshot()
	assert.InDelta(t, 1.5, snapshot.Totals.SpendUSD, 1e-9, "the unmetered row contributes nothing")

	wf := &model.Workflow{RunMaxUnits: 100, RunMaxWallClockSeconds: 10000, RunMaxBudgetUSD: 1.5}
	assert.Equal(t, StopCapBudget, capStop(&snapshot, wf))

	wf.RunMaxBudgetUSD = 0
	assert.Equal(t, StopReason(""), capStop(&snapshot, wf), "a budget of 0 is disabled")
}

// roundsRecorded returns how many review and audit rounds the spec's state
// index holds, which is what "no stop reason records a round" is measured on.
func roundsRecorded(t *testing.T, specPath string) (review, audit int) {
	t.Helper()
	st, err := LoadReviewState(specPath)
	require.NoError(t, err)
	if st == nil {
		return 0, 0
	}
	return len(st.ReviewRounds), len(st.AuditRounds)
}

// taskStatus reads one task's status from the task file on disk.
func taskStatus(t *testing.T, taskFile, id string) string {
	t.Helper()
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	for i := range tf.Tasks {
		if tf.Tasks[i].ID == id {
			return tf.Tasks[i].Status
		}
	}
	t.Fatalf("no task %q in %s", id, taskFile)
	return ""
}

// The wall-clock cap produces its own reason from a real run: the unit cap is
// nowhere near, so a stop recorded as cap-units here would be a cap reading a
// counter that is not its own.
func TestRunDriver_CapWallClockStopsTheRun(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, twoOpenTasks)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "1200")

	wf := driverWorkflow()
	wf.RunMaxWallClockSeconds = 1
	res := driveOnce(t, root, spec, taskFile, wf)

	assert.Equal(t, StopCapWallClock, res.StopReason)
	assert.Len(t, invocations(t, records), 1, "the cap stops the run after the iteration that reached it")
	assert.Equal(t, model.StatusOpen, taskStatus(t, taskFile, "beta"),
		"the cap closed no task of its own")
}


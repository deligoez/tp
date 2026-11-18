package engine

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecord_PutsAUnitsGateAndWaitTimeOnItsRow: the row carries three
// numbers, the attempt's whole duration and, apart from it, the seconds its
// gates ran and the seconds it waited for a slot, summed over its closes.
func TestRecord_PutsAUnitsGateAndWaitTimeOnItsRow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	taskFile := filepath.Join(root, "s.tasks.json")
	rec, err := NewRunRecorder(root, taskFile, "r1", "implement")
	require.NoError(t, err)
	require.NoError(t, rec.StartUnit(&RunUnitRow{Seq: 1, Kind: UnitImplement, ID: "alpha", Attempt: 1}))
	require.NoError(t, rec.StartUnit(&RunUnitRow{Seq: 2, Kind: UnitImplement, ID: "beta", Attempt: 1}))

	runDir := RunDir(root, "r1")
	path := GateTimesPath(runDir, 1)
	require.NoError(t, AddGateTimes(path, time.Now(), 0, 0))
	require.NoError(t, AddGateTimes(path, time.Time{}, 3*time.Second, 2*time.Second))
	require.NoError(t, AddGateTimes(path, time.Time{}, time.Second, 4*time.Second))

	d := &driver{opts: &DriverOptions{Root: root, TaskFile: taskFile}, runID: "r1", rec: rec}
	require.NoError(t, d.record([]*unitAttempt{
		{seq: 1, unit: NextUnit{Kind: UnitImplement, ID: "alpha"}, runner: &Runner{}, duration: 20 * time.Second},
		{seq: 2, unit: NextUnit{Kind: UnitImplement, ID: "beta"}, runner: &Runner{}, duration: 5 * time.Second},
	}))

	rows := rec.Snapshot().Units
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].GateSeconds, "the gate time is on the row")
	require.NotNil(t, rows[0].WaitSeconds, "the wait time is on the row")
	assert.InDelta(t, 6.0, *rows[0].GateSeconds, 1e-9, "summed over both closes")
	assert.InDelta(t, 4.0, *rows[0].WaitSeconds, 1e-9)
	assert.InDelta(t, 20.0, *rows[0].DurationSeconds, 1e-9, "the unit's own time is the whole attempt")
	assert.Nil(t, rows[1].GateSeconds, "a unit that ran no gate under a cap reports none")
	assert.Nil(t, rows[1].WaitSeconds)
}

// TestCloseTask_PutsTheMainGatesWaitInTheRunTotals: a main-branch close runs
// after its unit's row is finished, so its wait is a run-level figure: the
// waits summed and the longest one.
func TestCloseTask_PutsTheMainGatesWaitInTheRunTotals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	taskFile := filepath.Join(root, "s.tasks.json")
	rec, err := NewRunRecorder(root, taskFile, "r1", "implement")
	require.NoError(t, err)
	runDir := RunDir(root, "r1")
	waits := map[string]time.Duration{"alpha": 3 * time.Second, "beta": 5 * time.Second}
	closeFn := func(id, _ string, _ []string, _ string) error {
		// What the close's own tp done leaves behind for the driver.
		return AddGateTimes(MainGateTimesPath(runDir, id), time.Time{}, waits[id], time.Second)
	}
	d := &driver{opts: &DriverOptions{Root: root, TaskFile: taskFile, CloseTask: closeFn}, runID: "r1", rec: rec}
	for _, id := range []string{"alpha", "beta", "gamma"} {
		require.NoError(t, d.closeTask(&PendingClose{ID: id}, nil, filepath.Join(runDir, "integrate-"+id+".log")))
	}

	st := rec.Snapshot()
	assert.InDelta(t, 8.0, st.Totals.MainGateWaitSeconds, 1e-9, "the main gates' waits, summed")
	assert.InDelta(t, 5.0, st.Totals.MainGateWaitMaxSeconds, 1e-9, "the longest single wait")
	assert.Len(t, st.MainGates, 3, "gamma waited 0s, and still counts as a main gate")
}

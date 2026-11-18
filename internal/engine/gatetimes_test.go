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
	require.NoError(t, AddGateTimes(runDir, 1, time.Now(), 0, 0))
	require.NoError(t, AddGateTimes(runDir, 1, time.Time{}, 3*time.Second, 2*time.Second))
	require.NoError(t, AddGateTimes(runDir, 1, time.Time{}, time.Second, 4*time.Second))

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

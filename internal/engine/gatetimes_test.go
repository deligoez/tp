package engine

import (
	"path/filepath"
	"sync"
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

// TestAddGateTimes_ConcurrentAddsLoseNothing: a unit's background tp gate and
// its tp done can add to one time file at once; each add is read, summed and
// written under the file's lock, so none overwrites another.
func TestAddGateTimes_ConcurrentAddsLoseNothing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gate-1.json")
	const adders = 40
	var wg sync.WaitGroup
	for range adders {
		wg.Go(func() {
			assert.NoError(t, AddGateTimes(path, time.Time{}, 0, time.Second))
		})
	}
	wg.Wait()
	gt, ok := ReadGateTimes(path)
	require.True(t, ok)
	assert.InDelta(t, float64(adders), gt.GateSeconds, 0.001, "every add is counted")
}

// TestReadGateTimes_AWaitCutShortCounts: a taker killed while it waited
// (SIGKILL, which tp cannot catch) leaves waiting_since set; the driver reads
// the file once the taker has exited, so that wait counts up to the read.
func TestReadGateTimes_AWaitCutShortCounts(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gate-main-t1.json")
	require.NoError(t, AddGateTimes(path, time.Now().Add(-3*time.Second), 0, 0))

	gt, ok := ReadGateTimes(path)
	require.True(t, ok)
	assert.GreaterOrEqual(t, gt.WaitedSeconds, 3.0, "the cut-short wait counts")
	assert.Empty(t, gt.WaitingSince, "the read closes the open wait")
}

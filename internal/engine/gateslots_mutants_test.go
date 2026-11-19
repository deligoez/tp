package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcquireGateSlot_CapZeroIsNoCap: a cap of 0 is no cap. The gate starts at
// once, holding nothing, and no slot directory is made.
func TestAcquireGateSlot_CapZeroIsNoCap(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "gate-slots")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	slot, err := AcquireGateSlot(ctx, dir, 0, false)
	require.NoError(t, err, "a cap of 0 never waits")
	slot.Release()
	assert.NoDirExists(t, dir)
}

// TestRecord_AnUnrecordableResultIsTheDriversError: an exited attempt whose
// row cannot take its result is a driver error, named for the unit, not a
// result silently lost.
func TestRecord_AnUnrecordableResultIsTheDriversError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	taskFile := filepath.Join(root, "s.tasks.json")
	rec, err := NewRunRecorder(root, taskFile, "r1", "implement")
	require.NoError(t, err)

	d := &driver{opts: &DriverOptions{Root: root, TaskFile: taskFile}, runID: "r1", rec: rec}
	err = d.record([]*unitAttempt{{seq: 7, unit: NextUnit{Kind: UnitImplement, ID: "alpha"}, runner: &Runner{}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unit "alpha"`)
}

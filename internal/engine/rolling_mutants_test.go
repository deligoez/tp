package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestNextUnits_OneFreeSlotBesideOneRunningUnitGetsThePoolBrief: one unit
// runs and one slot is free. The unit that fills it is claimed by the driver,
// so its brief names its own task, as every unit beside a running pool does.
func TestNextUnits_OneFreeSlotBesideOneRunningUnitGetsThePoolBrief(t *testing.T) {
	root, spec, taskFile := setupResumeProject(t, rollingTasks(2, "alpha", "beta"))
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	tf.Tasks[0].Status = model.StatusWIP

	units, _ := BuildNextUnitsRunning(root, taskFile, spec, PhaseImplement, tf, nil, nil, []string{"alpha"})
	require.Equal(t, []string{"beta"}, unitIDs(units))
	assert.Equal(t, PoolBriefCommand("beta"), units[0].BriefCommand)
}

// TestRunRecorder_LeavingThePoolKeepsTheOtherOrphans: two orphans were carried
// in and one is integrated. The other is still waited for, so its pid stays.
func TestRunRecorder_LeavingThePoolKeepsTheOtherOrphans(t *testing.T) {
	root, _, taskFile := setupResumeProject(t, rollingTasks(2, "alpha", "beta"))
	rec, err := NewRunRecorder(root, taskFile, "r", "implement")
	require.NoError(t, err)
	require.NoError(t, rec.JoinOrphan("alpha", 101))
	require.NoError(t, rec.JoinOrphan("beta", 102))

	require.NoError(t, rec.LeavePool("alpha"))
	assert.Equal(t, map[string]int{"beta": 102}, rec.Snapshot().Orphans)
}

// TestUnitPID_ARowWithoutAPidFallsBackToTheOrphansPid: the unit's only row
// carries no pid, so its pid is the one the orphan was carried in with.
func TestUnitPID_ARowWithoutAPidFallsBackToTheOrphansPid(t *testing.T) {
	st := &RunState{
		Units:   []RunUnitRow{{Seq: 1, Kind: UnitImplement, ID: "alpha"}},
		Orphans: map[string]int{"alpha": 4242},
	}
	assert.Equal(t, 4242, unitPID(st, "alpha"))
}

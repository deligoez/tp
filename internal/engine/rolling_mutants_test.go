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


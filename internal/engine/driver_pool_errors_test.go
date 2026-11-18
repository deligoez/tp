package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// TestPool_ADriverErrorAfterASetupFailureIsTheStop: beta's setup fails, which
// drains the pool; alpha, still running, then fails to integrate as a driver
// error. The driver error is the more severe and is the stop; the setup
// failure is still named.
func TestPool_ADriverErrorAfterASetupFailureIsTheStop(t *testing.T) {
	root, spec, taskFile, _ := setupBatchProject(t, `test "$TP_UNIT_ID" != beta`)
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=800,beta=100")
	opts := rollingOptions(root, spec, taskFile, driverWorkflow())
	opts.CloseTask = func(id, _ string, _ []string, logPath string) error {
		return &CloseRefusedError{ID: id, LogPath: logPath}
	}

	res, err := RunDriver(opts)
	require.Error(t, err)
	assert.Equal(t, StopDriverError, res.StopReason, "the integration's driver error outranks the setup failure")
	require.NotNil(t, res.WorktreeSetup, "the setup failure is still named")
	assert.Equal(t, "beta", res.WorktreeSetup.ID)
}

package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// driveExpectingError runs the loop and returns what it ended as, without
// requiring success. driveOnce cannot serve here: it fails the test on any
// error, and a driver-side fatal error is exactly what these tests drive at.
func driveExpectingError(t *testing.T, root, spec, taskFile string, wf *model.Workflow) (DriverResult, error) {
	t.Helper()
	type outcome struct {
		res DriverResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := RunDriver(&DriverOptions{Root: root, TaskFile: taskFile, Spec: spec, Workflow: *wf})
		done <- outcome{res, err}
	}()
	select {
	case got := <-done:
		return got.res, got.err
	case <-time.After(90 * time.Second):
		t.Fatal("the driver loop did not terminate")
		return DriverResult{}, nil
	}
}

// §3.4 test 70, first arm: a runner that cannot be exec'd at all stops the run
// with driver-error, and the unit is not charged a failed attempt for it.
//
// The seam is pointed at a path that does not exist, so exec fails before any
// child exists — the one failure mode a non-zero exit code cannot be confused
// with. run_max_unit_retries is deliberately set high: if the driver charged
// this to the unit, the retry loop would spawn again and the run-state rows
// would count more than one attempt, which is the assertion that discriminates.
func TestRunDriver_ARunnerThatCannotExecStopsWithDriverError(t *testing.T) {
	root, spec, taskFile := setupResumeProject(t, oneOpenTask)
	t.Setenv(EnvRunnerSeam, filepath.Join(t.TempDir(), "no-such-runner"))

	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 2
	res, err := driveExpectingError(t, root, spec, taskFile, wf)

	var driverErr *DriverError
	require.ErrorAs(t, err, &driverErr,
		"a runner that will not exec is a driver-side fatal error, not an ordinary failure")
	assert.Equal(t, StopDriverError, res.StopReason,
		"the run stops with driver-error")

	st := readRunStateFile(t, root, taskFile)
	require.NotNil(t, st.StopReason, "the stop reason is recorded, not left as a crashed run")
	assert.Equal(t, StopDriverError, *st.StopReason)

	require.Len(t, st.Units, 1,
		"no unit is charged a failed attempt: the budget of 3 buys no retry for a driver-side failure")
	assert.Nil(t, st.Units[0].ExitCode,
		"a child that never ran has no exit code; reporting 0 would file the driver's own failure as a child that succeeded")
}

// §3.4 test 70, second arm: a run directory the driver cannot write is the same
// class of failure and takes the same stop reason.
//
// The root is placed under a regular file, so MkdirAll fails with ENOTDIR for
// every user including root — a permission bit would not fail under a uid 0
// test runner.
func TestRunDriver_ARunDirectoryItCannotWriteIsADriverError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	root := filepath.Join(blocker, "root")

	_, err := RunDriver(&DriverOptions{
		Root:     root,
		TaskFile: filepath.Join(root, "s.tasks.json"),
		Spec:     filepath.Join(root, "s.md"),
		Workflow: *driverWorkflow(),
	})

	var driverErr *DriverError
	require.ErrorAs(t, err, &driverErr,
		"a run directory the driver cannot write is a driver-side fatal error")
	assert.NotEmpty(t, driverErr.Hint(), "a driver error carries an actionable hint")
	assert.NotNil(t, driverErr.Unwrap(),
		"the filesystem cause is wrapped through, not replaced by the classification")
	assert.Contains(t, err.Error(), "run directory",
		"the message names what the driver could not do")
}

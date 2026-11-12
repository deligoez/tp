//go:build unix

package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestProcessAliveIsNotFooledByAReapBetweenItsTwoProbes drives the window that
// failed TestHookBoundJudgesCPUNotWallClock on a loaded CI runner in 5.00s: a
// killed child is a zombie when kill(pid, 0) answers, and its reaper collects
// it before /proc/<pid>/stat is read. A process that is gone must not be
// reported running because its stat file went with it.
func TestProcessAliveIsNotFooledByAReapBetweenItsTwoProbes(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	require.NoError(t, cmd.Process.Kill())

	reapThenRead := func(path string) ([]byte, error) {
		_ = cmd.Wait()
		return os.ReadFile(path) //nolint:gosec // the /proc path processAlive builds
	}
	require.False(t, processAliveReading(cmd.Process.Pid, reapThenRead),
		"the process was reaped before its stat was read; it is gone, not running")
}

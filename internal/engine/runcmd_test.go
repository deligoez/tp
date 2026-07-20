package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_Success_Failure_Timeout(t *testing.T) {
	dir := t.TempDir()

	t.Run("exit 0 passes", func(t *testing.T) {
		res := RunCommand("echo hello", dir, 10*time.Second, 20)
		assert.True(t, res.Passed)
		require.NotNil(t, res.ExitCode)
		assert.Equal(t, 0, *res.ExitCode)
		assert.False(t, res.TimedOut)
		assert.Equal(t, []string{"hello"}, res.OutputTail)
	})

	t.Run("non-zero exit fails with output tail", func(t *testing.T) {
		res := RunCommand("echo oops >&2; exit 3", dir, 10*time.Second, 20)
		assert.False(t, res.Passed)
		require.NotNil(t, res.ExitCode)
		assert.Equal(t, 3, *res.ExitCode)
		assert.False(t, res.TimedOut)
		assert.Equal(t, []string{"oops"}, res.OutputTail)
	})

	t.Run("tail size is parameterized", func(t *testing.T) {
		res := RunCommand("printf 'l1\\nl2\\nl3\\n'; exit 1", dir, 10*time.Second, 2)
		assert.Equal(t, []string{"l2", "l3"}, res.OutputTail)
	})

	t.Run("runs in caller cwd with inherited env", func(t *testing.T) {
		t.Setenv("TP_RUNCMD_TEST", "inherited")
		res := RunCommand("pwd; printf '%s\\n' \"$TP_RUNCMD_TEST\"", dir, 10*time.Second, 20)
		require.True(t, res.Passed)
		require.Len(t, res.OutputTail, 2)
		assert.Contains(t, res.OutputTail[0], dir)
		assert.Equal(t, "inherited", res.OutputTail[1])
	})

	t.Run("timeout counts as failure with timed-out message", func(t *testing.T) {
		res := RunCommand("sleep 5", dir, 1*time.Second, 20)
		assert.False(t, res.Passed)
		assert.True(t, res.TimedOut)
		assert.Nil(t, res.ExitCode)
		assert.Contains(t, res.Message, "timed out after 1s")
	})
}

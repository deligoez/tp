package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// §3.4 test 70: a driver-side fatal error — a runner that will not exec, a run
// directory the driver cannot write — is a STATE error (exit 4), the same class
// the run-lock precedent uses: the run really started and the filesystem or the
// harness is not in the shape it needs, so the answer is to hand the run to a
// human rather than to re-read the configuration. It is not the exit 2 §3.2's
// shape errors take, and not the code-1 default.
//
// The error is produced by driving a real driver into a run directory it cannot
// create, rather than hand-built, so the mapping is asserted over the error tp
// really returns. The classification goes through dispatchError because Execute
// itself ends in os.Exit.
func TestDispatchError_DriverErrorIsState(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	root := filepath.Join(blocker, "root")

	_, err := engine.RunDriver(&engine.DriverOptions{
		Root:     root,
		TaskFile: filepath.Join(root, "s.tasks.json"),
		Spec:     filepath.Join(root, "s.md"),
	})
	require.Error(t, err, "a run directory under a regular file cannot be created")

	code, msg, hint := dispatchError(err)
	assert.Equal(t, ExitState, code, "a driver-side fatal error exits 4, not %d", code)
	assert.Equal(t, err.Error(), msg, "the message names what the driver could not do")
	assert.NotEqual(t, runtimeFailureHint, hint,
		"a driver error carries its own hint, not the code-1 default that blames the task file")

	wrappedCode, _, _ := dispatchError(fmt.Errorf("driving the run: %w", err))
	assert.Equal(t, ExitState, wrappedCode, "a wrapped driver error is still a state error")
}

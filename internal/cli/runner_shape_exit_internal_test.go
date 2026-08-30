package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// A runner value that is none of §3.2's three shapes is a USAGE error, not a
// state error and not the code-1 default: nothing about the run state is wrong,
// the configuration simply does not say what to spawn, so no retry and no lock
// wait can change the answer. The classification is asserted through
// dispatchError because Execute itself ends in os.Exit.
func TestDispatchError_RunnerShapeIsUsage(t *testing.T) {
	// Test 56's two named usage errors, produced by the resolver rather than
	// hand-built, so the mapping is asserted over the errors tp really returns.
	for _, raw := range []string{
		`{"audit-role": "opencode"}`, // a map without default
		`{"default": {"args": []}}`,  // a runner object without cmd
		`{"cmd": "  "}`,              // the same, at the top level
	} {
		_, err := engine.ResolveRunner(json.RawMessage(raw), engine.UnitImplement)
		require.Error(t, err, "%s is none of the three shapes", raw)

		code, msg, hint := dispatchError(err)
		assert.Equal(t, ExitUsage, code, "%s exits 2, not %d", raw, code)
		assert.Equal(t, err.Error(), msg, "the message names the place at fault")
		assert.NotEmpty(t, hint, "a usage error carries an actionable hint")
		assert.NotEqual(t, runtimeFailureHint, hint,
			"the shape hint names the three shapes, not the exit-1 default")
	}
}

// A shape error stays a usage error through a wrapping chain: the driver that
// resolves a runner is several frames below the dispatcher, and %w is how it
// gets there.
func TestDispatchError_RunnerShapeSurvivesWrapping(t *testing.T) {
	_, err := engine.ResolveRunner(json.RawMessage(`{}`), engine.UnitAuditRole)
	require.Error(t, err)

	code, _, hint := dispatchError(fmt.Errorf("spawning audit-role: %w", err))
	assert.Equal(t, ExitUsage, code, "a wrapped shape error is still a usage error")

	var shapeErr *engine.RunnerShapeError
	require.ErrorAs(t, err, &shapeErr)
	assert.Equal(t, shapeErr.Hint(), hint, "the hint survives the wrapping too")
}

// The branch added for the shape error must not have moved any neighbour: the
// two lock errors are still state errors and everything else is still the
// code-1 default.
func TestDispatchError_NeighbouringClassificationsUnchanged(t *testing.T) {
	lockCode, _, lockHint := dispatchError(&engine.LockTimeoutError{LockPath: "/tmp/x.lock", Elapsed: time.Second})
	assert.Equal(t, ExitState, lockCode, "a write-lock timeout is still a state error")
	assert.NotEmpty(t, lockHint)

	runCode, _, runHint := dispatchError(&engine.RunLockBusyError{LockPath: "/tmp/run-x.lock", Base: "x"})
	assert.Equal(t, ExitState, runCode, "run-lock contention is still a state error")
	assert.NotEmpty(t, runHint)

	otherCode, otherMsg, otherHint := dispatchError(errors.New("something else failed"))
	assert.Equal(t, ExitValidation, otherCode, "an unclassified error is still exit 1")
	assert.Equal(t, "something else failed", otherMsg)
	assert.Equal(t, runtimeFailureHint, otherHint)
}

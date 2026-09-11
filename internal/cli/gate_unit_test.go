package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

func TestGateFailureMessage_TimeoutReportsSecondsAndNullExitCode(t *testing.T) {
	t.Parallel()
	wf := &model.Workflow{QualityGate: "sleep 999", GateTimeoutSeconds: 45}
	res := engine.RunResult{TimedOut: true}

	assert.Equal(t, "gate timed out after 45s", gateFailureMessage(wf, res))

	data, err := json.Marshal(map[string]any{"exit_code": res.ExitCode})
	require.NoError(t, err)
	assert.JSONEq(t, `{"exit_code": null}`, string(data), "timeout serializes exit_code as null")
}

func TestGateFailureMessage_NonZeroExitNamesGateCmd(t *testing.T) {
	t.Parallel()
	wf := &model.Workflow{QualityGate: "go test ./...", GateTimeoutSeconds: 600}
	code := 1
	res := engine.RunResult{ExitCode: &code}

	assert.Equal(t, "quality gate failed: go test ./...", gateFailureMessage(wf, res))
}

// A gate that exited 127 with nothing to name still steers to the gate
// command, never to --skip-gate; a timeout carries no exit code and keeps the
// ordinary hint.
func TestGateFailureText_CannotRunWithoutANameStillAvoidsSkipGate(t *testing.T) {
	t.Parallel()
	wf := &model.Workflow{QualityGate: "true", GateTimeoutSeconds: 600}
	code := 127
	msg, hint := gateFailureText(wf, engine.RunResult{ExitCode: &code}, t.TempDir())
	assert.Equal(t, "quality gate could not run one of its commands (exit 127: command not found)", msg)
	assert.Contains(t, hint, "fix the gate command")
	assert.NotContains(t, hint, "--skip-gate")

	msg, hint = gateFailureText(wf, engine.RunResult{TimedOut: true}, t.TempDir())
	assert.Equal(t, "gate timed out after 600s", msg)
	assert.Equal(t, gateSkipHint, hint)
}

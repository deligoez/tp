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
	wf := &model.Workflow{QualityGate: "sleep 999", GateTimeoutSeconds: 45}
	res := engine.RunResult{TimedOut: true}

	assert.Equal(t, "gate timed out after 45s", gateFailureMessage(wf, res))

	data, err := json.Marshal(map[string]any{"exit_code": res.ExitCode})
	require.NoError(t, err)
	assert.JSONEq(t, `{"exit_code": null}`, string(data), "timeout serializes exit_code as null")
}

func TestGateFailureMessage_NonZeroExitNamesGateCmd(t *testing.T) {
	wf := &model.Workflow{QualityGate: "go test ./...", GateTimeoutSeconds: 600}
	code := 1
	res := engine.RunResult{ExitCode: &code}

	assert.Equal(t, "quality gate failed: go test ./...", gateFailureMessage(wf, res))
}

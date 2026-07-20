package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowUnmarshal_Defaults(t *testing.T) {
	var w Workflow
	require.NoError(t, json.Unmarshal([]byte(`{}`), &w))

	assert.Equal(t, "", w.QualityGate)
	assert.Equal(t, 2, w.ReviewCleanRounds)
	assert.Equal(t, 2, w.AuditCleanRounds)
	assert.Equal(t, 600, w.GateTimeoutSeconds)
	assert.Equal(t, []Check{}, w.Checks)
	assert.Equal(t, 0, w.ReviewMaxRounds)
	assert.Equal(t, 0, w.AuditMaxRounds)
}

func TestWorkflowUnmarshal_ExplicitValuesPreserved(t *testing.T) {
	var w Workflow
	require.NoError(t, json.Unmarshal([]byte(`{
		"gate_timeout_seconds": 0,
		"checks": [{"class": "nil-slice", "cmd": "grep -rn 'var .* \\[\\]' internal/"}],
		"review_max_rounds": 5,
		"audit_max_rounds": 50
	}`), &w))

	assert.Equal(t, 0, w.GateTimeoutSeconds, "explicit zero preserved for validation to reject")
	assert.Equal(t, []Check{{Class: "nil-slice", Cmd: "grep -rn 'var .* \\[\\]' internal/"}}, w.Checks)
	assert.Equal(t, 5, w.ReviewMaxRounds)
	assert.Equal(t, 50, w.AuditMaxRounds)
}

// TestWorkflow_FieldTableSerialization covers the 6.7 field table: every row
// round-trips through JSON with its stated default.
func TestWorkflow_FieldTableSerialization(t *testing.T) {
	var w Workflow
	require.NoError(t, json.Unmarshal([]byte(`{}`), &w))

	data, err := json.Marshal(w)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, `"quality_gate"`, "empty quality_gate omitted (default \"\")")
	assert.Contains(t, raw, `"gate_timeout_seconds":600`)
	assert.Contains(t, raw, `"checks":[]`)
	assert.Contains(t, raw, `"review_max_rounds":0`)
	assert.Contains(t, raw, `"audit_max_rounds":0`)

	var got Workflow
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, w, got)
}

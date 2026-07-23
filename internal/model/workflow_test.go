package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Workflow is the resolved (effective) workflow value the resolver produces and
// tp plan / validate / config emit. As of v0.26.0 it applies no defaults on
// unmarshal — defaults live only in the resolver (DefaultWorkflow) — so this
// checks the effective value marshals with every field present and round-trips
// without any default injection.
func TestWorkflow_EffectiveSerialization(t *testing.T) {
	w := Workflow{
		QualityGate:        "go test ./...",
		ReviewCleanRounds:  3,
		AuditCleanRounds:   2,
		GateTimeoutSeconds: 600,
		LockTimeoutSeconds: 5,
		Checks:             []Check{},
		ReviewMaxRounds:    0,
		AuditMaxRounds:     0,
	}

	data, err := json.Marshal(w)
	require.NoError(t, err)

	raw := string(data)
	assert.Contains(t, raw, `"quality_gate":"go test ./..."`)
	assert.Contains(t, raw, `"review_clean_rounds":3`)
	assert.Contains(t, raw, `"gate_timeout_seconds":600`)
	assert.Contains(t, raw, `"lock_timeout_seconds":5`)
	assert.Contains(t, raw, `"checks":[]`)
	assert.Contains(t, raw, `"review_max_rounds":0`)

	var got Workflow
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, w, got, "the effective workflow round-trips without default injection")
}

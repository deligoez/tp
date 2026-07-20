package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateSkippedReasonField(t *testing.T) {
	t.Run("omitted when nil", func(t *testing.T) {
		task := Task{ID: "t1", Title: "T", Status: StatusOpen, EstimateMinutes: 5, Acceptance: "ok"}
		data, err := json.Marshal(task)
		require.NoError(t, err)
		assert.False(t, strings.Contains(string(data), "gate_skipped_reason"))
	})

	t.Run("round-trips when set", func(t *testing.T) {
		reason := "env broken, user approved"
		task := Task{ID: "t1", Title: "T", Status: StatusDone, EstimateMinutes: 5, Acceptance: "ok", GateSkippedReason: &reason}
		data, err := json.Marshal(task)
		require.NoError(t, err)
		assert.True(t, strings.Contains(string(data), `"gate_skipped_reason":"env broken, user approved"`))

		var back Task
		require.NoError(t, json.Unmarshal(data, &back))
		require.NotNil(t, back.GateSkippedReason)
		assert.Equal(t, reason, *back.GateSkippedReason)
	})
}

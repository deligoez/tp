package engine

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// §3.4's table, transcribed. A run stops for exactly one of these reasons and
// records it verbatim, so the values are pinned here rather than derived: a
// constant that drifts from the spec's spelling is a stop nobody can match on.
func TestStopReason_IsSection34sNineValuesVerbatim(t *testing.T) {
	cases := []struct {
		reason StopReason
		want   string
	}{
		{StopConverged, "converged"},
		{StopCapUnits, "cap-units"},
		{StopCapWallClock, "cap-wall-clock"},
		{StopCapBudget, "cap-budget"},
		{StopEscalation, "escalation"},
		{StopUnitFailure, "unit-failure"},
		{StopNoUnits, "no-units"},
		{StopInterrupted, "interrupted"},
		{StopDriverError, "driver-error"},
	}
	require.Len(t, cases, 9, "§3.4 names nine stop reasons")

	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		assert.Equal(t, tc.want, string(tc.reason))
		assert.True(t, tc.reason.Known(), "%s is one of the nine", tc.want)
		assert.False(t, seen[tc.want], "%s is declared once", tc.want)
		seen[tc.want] = true
	}
}


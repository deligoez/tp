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

// The vocabulary is closed: anything that is not one of the nine — including
// the empty string capStop returns while a run is within every cap — is not a
// stop reason.
func TestStopReason_KnownRejectsEverythingElse(t *testing.T) {
	for _, near := range []StopReason{"", "cap_units", "CONVERGED", "capunits", "cap-unit", "done", "converged "} {
		assert.False(t, near.Known(), "%q is not one of §3.4's nine", string(near))
	}
}

// The guard sits at the sink: the recorder is the one writer of stop_reason,
// so a reason outside the vocabulary is refused there rather than trusted from
// whichever caller composed it, and the file keeps whatever it already held.
func TestRunRecorder_StopRefusesAReasonOutsideTheVocabulary(t *testing.T) {
	root := t.TempDir()
	rec, err := NewRunRecorder(root, "s.tasks.json", "run", PhaseImplement)
	require.NoError(t, err)

	require.NoError(t, rec.Stop(StopCapUnits))
	require.Error(t, rec.Stop("cap_units"), "an unrecognized reason is never recorded")

	st, err := ReadRunState(root, "s.tasks.json")
	require.NoError(t, err)
	require.NotNil(t, st.StopReason)
	assert.Equal(t, StopCapUnits, *st.StopReason, "the refused write left the recorded reason alone")
}


package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeEscalationJSON writes a record exactly as given, bypassing
// WriteEscalation's normalizations: a broken record has to reach disk broken,
// and tp's own writer stamps At and replaces a nil Options.
func writeEscalationJSON(t *testing.T, path string, e *Escalation) {
	t.Helper()
	data, err := json.Marshal(e)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(data, '\n'), 0o600))
}

// validEscalation is the record `tp escalate` writes: every field §5.2
// documents, filled the way a unit under the driver fills it.
func validEscalation() *Escalation {
	return &Escalation{
		Decision: EscalateSkipGate,
		UnitKind: UnitImplement,
		UnitID:   "alpha",
		Phase:    PhaseImplement,
		Evidence: "the gate fails on a lint error the task did not introduce",
		Options:  []string{"fix the lint error first", "skip the gate for this task"},
		At:       "2026-08-30T09:00:00Z",
	}
}

// The record tp writes is the record the driver reads. Round-tripping it
// through the real writer is what keeps the two ends from drifting apart: a
// reader tested only against JSON a test hand-wrote can agree with the test and
// with nothing else.
func TestReadEscalation_ReadsTheRecordTPEscalateWrote(t *testing.T) {
	path, err := WriteEscalation(t.TempDir(), "7", validEscalation())
	require.NoError(t, err)

	got, err := ReadEscalation(path)
	require.NoError(t, err)
	assert.Equal(t, validEscalation(), got)
}


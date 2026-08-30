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

// §5.2: a record that fails schema validation is not an escalation, and the
// unit is judged by its predicate and its exit code instead. Each row is one
// documented field made unusable, because a validator that only rejects
// wholesale garbage would accept the record a truncated or hand-rolled writer
// produces — the record that would end a run the unit never meant to end.
func TestReadEscalation_RejectsARecordThatIsNotOne(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(e *Escalation)
	}{
		{"a decision outside the closed five", func(e *Escalation) { e.Decision = "invent-one" }},
		{"no decision at all", func(e *Escalation) { e.Decision = "" }},
		{"a unit kind outside the eight", func(e *Escalation) { e.UnitKind = "implementation" }},
		{"no unit kind", func(e *Escalation) { e.UnitKind = "" }},
		{"no unit id", func(e *Escalation) { e.UnitID = "" }},
		{"no phase", func(e *Escalation) { e.Phase = "" }},
		{"no evidence", func(e *Escalation) { e.Evidence = "" }},
		{"blank evidence", func(e *Escalation) { e.Evidence = "   " }},
		{"no timestamp", func(e *Escalation) { e.At = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := validEscalation()
			tc.breaks(record)
			// WriteEscalation stamps At when it is empty, so the broken
			// record is written as-is rather than through tp's own writer.
			path := filepath.Join(t.TempDir(), "1-escalation.json")
			writeEscalationJSON(t, path, record)

			got, err := ReadEscalation(path)
			require.Error(t, err, "a record missing a documented field is not a valid escalation")
			assert.Nil(t, got)
		})
	}
}

// options is documented as an array and WriteEscalation always emits one, so a
// record without it did not come from the documented path.
func TestReadEscalation_RejectsANullOptionsArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1-escalation.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"decision":"other","unit_kind":"implement",`+
		`"unit_id":"alpha","phase":"implement","evidence":"e","at":"2026-08-30T09:00:00Z"}`), 0o600))

	got, err := ReadEscalation(path)
	require.Error(t, err)
	assert.Nil(t, got)
}

// Absent, unreadable and unparseable are one answer with invalid, because §5.2
// gives all four the same consequence.
func TestReadEscalation_RejectsWhatIsNotARecordFile(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		content string
		write   bool
	}{
		{name: "no record at all"},
		{name: "an empty file", write: true},
		{name: "prose rather than JSON", content: "escalating: the gate failed\n", write: true},
		{name: "a JSON array", content: `[{"decision":"other"}]`, write: true},
		{name: "a truncated object", content: `{"decision":"other","unit_kind":`, write: true},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, string(rune('a'+i))+"-escalation.json")
			if tc.write {
				require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o600))
			}
			got, err := ReadEscalation(path)
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

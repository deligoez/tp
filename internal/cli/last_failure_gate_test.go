package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// unitEnv is the identity a unit tp run spawned carries (§3.1.1). The gate
// writer keys off TP_UNIT_KIND, so every case here either sets the trio or
// omits it entirely.
func gateUnitEnv(kind, id, phase string) []string {
	return []string{
		engine.EnvUnitKind + "=" + kind,
		engine.EnvUnitID + "=" + id,
		engine.EnvPhase + "=" + phase,
	}
}

// readGateLastFailure returns the record tp done left for the cycle, or nil.
// It reads the file directly rather than through ReadLastFailure so that an
// unparseable record fails the test instead of reading as absent.
func readGateLastFailure(t *testing.T, dir string) *engine.LastFailure {
	t.Helper()
	path := engine.LastFailurePath(dir, filepath.Join(dir, "spec.tasks.json"))
	data, err := os.ReadFile(path) //nolint:gosec // a path this test built
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	var record engine.LastFailure
	require.NoError(t, json.Unmarshal(data, &record), "the record must be valid JSON: %s", data)
	return &record
}

// §4.2, test 61: `tp done` is the second writer of last_failure. A gate the
// harness swallows is still recorded, so the next unit reads what this one
// walked into — and the summary is the gate's own output, not the closing
// agent's prose.
func TestDoneGate_FailureUnderARunRecordsLastFailure(t *testing.T) {
	// The gate prints a string its own command text does not contain, so an
	// assertion on the output cannot be satisfied by a summary that merely
	// echoes the command back.
	dir := setupProjectWithGate(t, `printf 'gate-tail-%s\n' marker; exit 7`)
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	reason := "task complete and verified fully"
	_, stderr, code := runTPEnv(t, dir, gateUnitEnv("implement", "t1", "implement"), "done", "t1", reason)
	require.Equal(t, 4, code, "gate failure still exits 4: %s", stderr)

	record := readGateLastFailure(t, dir)
	require.NotNil(t, record, "a failed gate under TP_UNIT_KIND records last_failure")
	assert.Equal(t, engine.UnitImplement, record.UnitKind, "unit_kind comes from TP_UNIT_KIND")
	assert.Equal(t, "t1", record.UnitID, "unit_id comes from TP_UNIT_ID")
	assert.Equal(t, "implement", record.Phase, "phase comes from TP_PHASE")
	assert.Equal(t, 7, record.ExitCode, "exit_code is the gate's own")
	assert.NotEmpty(t, record.At)
	assert.Contains(t, record.Summary, "gate-tail-marker", "summary carries the gate's own output")
	assert.Contains(t, record.Summary, `printf 'gate-tail-%s\n' marker; exit 7`, "summary names the gate that failed")
	assert.NotContains(t, record.Summary, reason, "the summary never copies the child's prose")
}


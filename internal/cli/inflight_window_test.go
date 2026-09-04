package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emitAuditRound sets up a spec with a task file and emits one audit round,
// leaving the state directory holding a snapshot and no index — the window
// between emitting a round and recording it.
func emitAuditRound(t *testing.T) (dir, specPath string) {
	t.Helper()
	dir = t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath,
		[]byte("# Spec\n\n## 1. Thing\n\n1. Do the thing.\n2. Do the other thing.\n"), 0o600))
	goPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(goPath, []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", specPath)
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "audit", specPath, "--affected-files", goPath)
	require.Equal(t, 0, code, "emission: %s", stderr)

	entries, err := os.ReadDir(filepath.Join(dir, ".tp-review", "spec"))
	require.NoError(t, err)
	require.Len(t, entries, 1, "the emission leaves exactly the snapshot, no index")
	return dir, specPath
}

// TestInFlightWindowIsReadableByEveryStateReader: an audit emission writes a
// round snapshot and never creates state.json, so between emitting a round and
// recording it the index is legitimately absent. Every command that reads that
// state has to agree the window is normal. Three did not, and the one that
// mattered most was tp resume — the reset-native orchestrator's re-orientation
// oracle — which exited 3 telling the caller to delete the directory holding
// its own in-flight round.
func TestInFlightWindowIsReadableByEveryStateReader(t *testing.T) {
	t.Parallel()
	dir, specPath := emitAuditRound(t)

	t.Run("tp resume", func(t *testing.T) {
		stdout, stderr, code := runTP(t, dir, "resume")
		require.Equal(t, 0, code, "resume must work in the window it exists to re-orient in: %s", stderr)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		assert.NotEmpty(t, result["phase"], "resume still answers with a phase")
	})

	t.Run("tp audit --status", func(t *testing.T) {
		stdout, stderr, code := runTP(t, dir, "audit", specPath, "--status")
		require.Equal(t, 0, code, "status: %s", stderr)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		assert.Equal(t, float64(1), result["in_flight_round"], "the emitted round is in flight")
	})

	t.Run("tp review --status", func(t *testing.T) {
		_, stderr, code := runTP(t, dir, "review", specPath, "--status")
		assert.Equal(t, 0, code, "review --status reads the same directory: %s", stderr)
	})

	t.Run("tp review emission", func(t *testing.T) {
		_, stderr, code := runTP(t, dir, "review", specPath)
		assert.Equal(t, 0, code, "a review round emits in the same window: %s", stderr)
	})

	t.Run("tp audit --record", func(t *testing.T) {
		results := filepath.Join(dir, "r1.ndjson")
		require.NoError(t, os.WriteFile(results, []byte(
			`{"item_id":"list-0-1","status":"PASS","evidence_file":"a.go","evidence_lines":"1","category":null,"severity":null,"notes":"","role":"spec-coverage","location":"§1"}`+"\n"), 0o600))
		stdout, stderr, code := runTP(t, dir, "audit", specPath, "--record", results)
		require.Equal(t, 0, code, "the first round of a fresh spec must be recordable: %s", stderr)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		assert.Equal(t, float64(1), result["round"])
	})
}

// TestLostRoundIndexStillAborts is the other half: a round file with no index is
// recorded history tp cannot see, and rebuilding over it would discard it. The
// emission path is the arm that matters — under the default audit_max_rounds=0
// the budget guard returns before reading state, so this once emitted a prompt
// and overwrote the round snapshot while --record on the same directory exited 3.
func TestLostRoundIndexStillAborts(t *testing.T) {
	t.Parallel()
	dir, specPath := emitAuditRound(t)
	goPath := filepath.Join(dir, "a.go")

	results := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(
		`{"item_id":"list-0-1","status":"FAIL","evidence_file":null,"evidence_lines":null,"category":"correctness","severity":"error","notes":"real finding","role":"go-safety","location":"§1"}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "record round 1: %s", stderr)

	stateDir := filepath.Join(dir, ".tp-review", "spec")
	roundFile := filepath.Join(stateDir, "audit-round-1.ndjson")
	require.FileExists(t, roundFile)
	before, err := os.ReadFile(roundFile)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(stateDir, "state.json")))

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"emission", []string{"audit", specPath, "--affected-files", goPath}},
		{"status", []string{"audit", specPath, "--status"}},
		{"record", []string{"audit", specPath, "--record", results}},
		{"resume", []string{"resume"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runTP(t, dir, tc.args...)
			assert.Equal(t, 3, code, "a round file with no index is lost history, not an in-flight round")
			assert.Contains(t, stderr, "state.json is missing")
		})
	}

	after, err := os.ReadFile(roundFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "no refused command may rewrite the recorded round")
}

// TestUnlistableStateDirIsNotReadAsAbsent: hasStateArtifacts answered its
// ReadDir error with false, so a state directory tp could not list read as a
// state that does not exist — and --record then built a fresh index over the
// round file already there, turning a recorded FAIL round into a clean round 1.
func TestUnlistableStateDirIsNotReadAsAbsent(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root lists a 0o300 directory, so the read never fails")
	}
	dir, specPath := emitAuditRound(t)

	results := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(
		`{"item_id":"list-0-1","status":"FAIL","evidence_file":null,"evidence_lines":null,"category":"correctness","severity":"error","notes":"real finding","role":"go-safety","location":"§1"}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "record round 1: %s", stderr)

	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.Remove(filepath.Join(stateDir, "state.json")))
	require.NoError(t, os.Chmod(stateDir, 0o300)) // writable, not listable
	t.Cleanup(func() { _ = os.Chmod(stateDir, 0o700) })

	_, _, code = runTP(t, dir, "audit", specPath, "--record", results)
	assert.Equal(t, 3, code, "a state tp cannot list is not a state tp may overwrite")

	require.NoError(t, os.Chmod(stateDir, 0o700))
	assert.FileExists(t, filepath.Join(stateDir, "audit-round-1.ndjson"),
		"the recorded round survives the refusal")
}

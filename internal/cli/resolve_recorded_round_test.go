package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedRoundRows is one recorded audit round: a PASS and one finding.
const recordedRoundRows = `{"role":"go-safety","item_id":"a","status":"PASS"}
{"role":"go-safety","item_id":"b","status":"FAIL","severity":"warning"}
`

// recordedAuditProject records recordedRoundRows as audit round 1 of spec.md
// and returns the project directory.
func recordedAuditProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "%s", stderr)
	_, stderr, code = auditRecord(t, dir, recordedRoundRows)
	require.Equal(t, 0, code, "%s", stderr)
	return dir
}

func decodePayload(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "%s", stdout)
	return out
}

// TestAuditResolve_IntoAFileNoRoundListsSaysSo: a disposition written into a
// results file no state.json lists changes no round's verdict. The resolve
// said nothing, and by hand it offered `tp audit <spec> --record <file>`,
// which appends a new round built from the old round's rows with no emission
// behind it. It now says the file is not a recorded round and offers nothing.
func TestAuditResolve_IntoAFileNoRoundListsSaysSo(t *testing.T) {
	t.Parallel()
	dir := recordedAuditProject(t)
	copyPath := filepath.Join(dir, "copy.ndjson")
	require.NoError(t, os.WriteFile(copyPath, []byte(recordedRoundRows), 0o600))

	stdout, stderr, code := runTPFence(t, dir, false, "audit", copyPath, "--resolve", "go-safety:b", "fixed", "done")
	require.Equal(t, 0, code, "%s", stderr)
	out := decodePayload(t, stdout)
	assert.Equal(t, false, out["recorded_round"])
	assert.NotContains(t, out, "next_step", "by hand, re-recording appends a round nothing emitted")
	assert.Contains(t, stderr, "not a recorded round; no round state changed")
}

// TestAuditResolve_IntoTheRecordedRoundSaysNothingExtra: the recorded round
// file is what the loop verdict reads, so a resolve there adds no statement.
func TestAuditResolve_IntoTheRecordedRoundSaysNothingExtra(t *testing.T) {
	t.Parallel()
	dir := recordedAuditProject(t)
	roundFile := filepath.Join(dir, ".tp-review", "spec", "audit-round-1.ndjson")

	stdout, stderr, code := runTPFence(t, dir, false, "audit", roundFile, "--resolve", "go-safety:b", "fixed", "done")
	require.Equal(t, 0, code, "%s", stderr)
	assert.NotContains(t, decodePayload(t, stdout), "recorded_round")
	assert.NotContains(t, stderr, "not a recorded round")
}

// TestAuditResolve_NextStepOnlyForTheDriverRoundsExactCopy: inside a driver
// round (TP_ROUND and TP_FILE set), the record unit rewrites that round in
// place, so re-recording is the next step, but only for an exact copy of the
// round's rows with dispositions set aside. A file that differs from the
// round would replace the recorded rows with others.
func TestAuditResolve_NextStepOnlyForTheDriverRoundsExactCopy(t *testing.T) {
	t.Parallel()
	dir := recordedAuditProject(t)
	env := []string{"TP_ROUND=1", "TP_FILE=" + filepath.Join(dir, "spec.tasks.json"), "TP_UNATTENDED="}

	exact := filepath.Join(dir, "merged.ndjson")
	require.NoError(t, os.WriteFile(exact, []byte(recordedRoundRows), 0o600))
	stdout, stderr, code := runTPEnv(t, dir, env, "audit", exact, "--resolve", "go-safety:b", "fixed", "done")
	require.Equal(t, 0, code, "%s", stderr)
	assert.Equal(t, "tp audit <spec> --record "+exact, decodePayload(t, stdout)["next_step"])

	other := filepath.Join(dir, "other.ndjson")
	require.NoError(t, os.WriteFile(other, []byte(recordedRoundRows+`{"role":"go-safety","item_id":"c","status":"FAIL"}`+"\n"), 0o600))
	stdout, stderr, code = runTPEnv(t, dir, env, "audit", other, "--resolve-all", "fixed", "done")
	require.Equal(t, 0, code, "%s", stderr)
	assert.NotContains(t, decodePayload(t, stdout), "next_step", "a file that is not the round's copy is not re-recorded")
}

// TestAuditResolveAll_SkipsPassRows: a disposition answers a finding, and a
// PASS row is not one. --resolve-all wrote a disposition onto every PASS row.
func TestAuditResolveAll_SkipsPassRows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(path, []byte(recordedRoundRows), 0o600))

	stdout, stderr, code := runTPFence(t, dir, false, "audit", path, "--resolve-all", "fixed", "done")
	require.Equal(t, 0, code, "%s", stderr)
	out := decodePayload(t, stdout)
	assert.Equal(t, float64(1), out["resolved_count"])
	assert.Equal(t, float64(0), out["skipped_count"], "a PASS row is not counted as skipped")
	rows := readAuditRows(t, path)
	assert.NotContains(t, rows[0], "resolved", "the PASS row takes no disposition")
	assert.Equal(t, "fixed", resolvedOf(t, rows[1])["status"])
}

// TestReviewResolve_IntoAFileNoRoundListsSaysSo: the review side makes the
// same statement.
func TestReviewResolve_IntoAFileNoRoundListsSaysSo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(path,
		[]byte(`{"location":"§1","class":"x","severity":"medium","finding":"f","suggestion":"s"}`+"\n"), 0o600))

	stdout, stderr, code := runTPFence(t, dir, false, "review", path, "--resolve", "0", "fixed", "done")
	require.Equal(t, 0, code, "%s", stderr)
	assert.Equal(t, false, decodePayload(t, stdout)["recorded_round"])
	assert.Contains(t, stderr, "not a recorded round; no round state changed")

	stdout, stderr, code = runTPFence(t, dir, false, "review", path, "--resolve-all", "fixed", "done", "--force")
	require.Equal(t, 0, code, "%s", stderr)
	assert.Equal(t, false, decodePayload(t, stdout)["recorded_round"])
	assert.Contains(t, stderr, "not a recorded round; no round state changed")
}

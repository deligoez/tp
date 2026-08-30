package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneOpenTaskJSON is the task array newPayloadRepo wraps: a single open task, so
// both surfaces have exactly one unit to put the record in front of.
const oneOpenTaskJSON = `[{"id":"a","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"acc","source_sections":["x"]}]`

// writeLastFailure plants §4.2's record for the repo's cycle, the way a driver
// or a failing gate would have left it behind for the next process.
func writeLastFailure(t *testing.T, dir string) map[string]any {
	t.Helper()
	record := map[string]any{
		"unit_kind": "implement",
		"unit_id":   "a",
		"phase":     "implement",
		"exit_code": 1,
		"summary":   "command: fake-runner implement a; log: /runs/1-implement-a.jsonl",
		"at":        "2026-08-30T00:00:00Z",
	}
	data, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "last_failure-spec.json"), data, 0o600))
	return record
}

// §4.2: tp resume surfaces last_failure when one is present, which is how a
// fresh process learns what the previous attempt walked into.
func TestResume_SurfacesLastFailure(t *testing.T) {
	dir := newPayloadRepo(t, oneOpenTaskJSON)
	want := writeLastFailure(t, dir)

	res := resumeResult(t, dir)
	got, ok := res["last_failure"].(map[string]any)
	require.True(t, ok, "tp resume carries the record as an object: %v", res["last_failure"])
	assert.Equal(t, "implement", got["unit_kind"])
	assert.Equal(t, "a", got["unit_id"])
	assert.Equal(t, float64(1), got["exit_code"])
	assert.Equal(t, want["summary"], got["summary"])
	assert.Equal(t, want["at"], got["at"])
}

// The record is the one piece of continuity the loop needs, so --compact keeps
// it whole: stripping it would strip the reason the next unit reads it at all.
func TestResume_LastFailureSurvivesCompact(t *testing.T) {
	dir := newPayloadRepo(t, oneOpenTaskJSON)
	writeLastFailure(t, dir)

	out, stderr, code := runTP(t, dir, "resume", "--compact")
	require.Equal(t, 0, code, "resume --compact: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))

	got, ok := res["last_failure"].(map[string]any)
	require.True(t, ok, "--compact keeps the record")
	assert.Equal(t, "command: fake-runner implement a; log: /runs/1-implement-a.jsonl", got["summary"])
}

// §4.2: the record is advisory. A cycle that never failed reports it as null
// rather than omitting the key, so the driver's parse of tp resume has one
// shape whatever happened.
func TestResume_LastFailureIsNullWithoutARecord(t *testing.T) {
	dir := newPayloadRepo(t, oneOpenTaskJSON)

	res := resumeResult(t, dir)
	require.Contains(t, res, "last_failure")
	assert.Nil(t, res["last_failure"])
}

// §4.2: tp brief surfaces the record too, which is what actually puts it in
// front of the next unit — the brief is what a spawned unit reads first.
func TestBrief_SurfacesLastFailure(t *testing.T) {
	dir := newPayloadRepo(t, oneOpenTaskJSON)
	want := writeLastFailure(t, dir)

	out, stderr, code := runTP(t, dir, "brief", "a")
	require.Equal(t, 0, code, "brief: %s", stderr)
	var b map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &b))

	got, ok := b["last_failure"].(map[string]any)
	require.True(t, ok, "tp brief carries the record: %v", b["last_failure"])
	assert.Equal(t, "a", got["unit_id"])
	assert.Equal(t, want["summary"], got["summary"])
}

// A brief for a cycle that never failed is unchanged: the section and the key
// appear only when there is something to report, so the common brief costs the
// unit no extra bytes (P3).
func TestBrief_OmitsLastFailureWithoutARecord(t *testing.T) {
	dir := newPayloadRepo(t, oneOpenTaskJSON)

	out, stderr, code := runTP(t, dir, "brief", "a")
	require.Equal(t, 0, code, "brief: %s", stderr)
	var b map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &b))

	assert.NotContains(t, b, "last_failure")
}

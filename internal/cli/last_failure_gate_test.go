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

// §4.2: outside a run the record's unit_kind, unit_id and phase have no value,
// so tp done writes nothing at all — the shipped command is unchanged there,
// error shape included.
func TestDoneGate_FailureOutsideARunRecordsNothing(t *testing.T) {
	dir := setupProjectWithGate(t, "echo boom; exit 7")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully")
	require.Equal(t, 4, code, "gate failure still exits 4: %s", stderr)

	assert.Nil(t, readGateLastFailure(t, dir),
		"no TP_UNIT_KIND, no record: there is no unit to attribute the failure to")

	var errOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &errOut), "stderr: %s", stderr)
	assert.Equal(t, "echo boom; exit 7", errOut["gate_cmd"], "the shipped error object is unchanged")
	assert.Equal(t, float64(7), errOut["exit_code"])
}

// The two triggers are a failed gate, not any gate: a gate that passes under
// the same environment records nothing, so the file cannot fill up with
// successes and the record stays the one thing §4.2 says it is.
func TestDoneGate_PassingGateUnderARunRecordsNothing(t *testing.T) {
	dir := setupProjectWithGate(t, "echo ok")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTPEnv(t, dir, gateUnitEnv("implement", "t1", "implement"), "done", "t1", "task complete and verified fully")
	require.Equal(t, 0, code, "done failed: %s", stderr)

	assert.Nil(t, readGateLastFailure(t, dir), "a passing gate records nothing")
}

// A gate with no output of its own — a timeout, or a silent non-zero exit —
// still produces a summary that says what failed, because a record whose
// summary is empty tells the next unit nothing.
func TestDoneGate_SilentGateFailureStillSummarizes(t *testing.T) {
	dir := setupProjectWithGate(t, "exit 3")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, _, code := runTPEnv(t, dir, gateUnitEnv("implement", "t1", "implement"), "done", "t1", "task complete and verified fully")
	require.Equal(t, 4, code)

	record := readGateLastFailure(t, dir)
	require.NotNil(t, record)
	assert.Equal(t, 3, record.ExitCode)
	assert.NotEmpty(t, strings.TrimSpace(record.Summary), "a silent gate still gets a summary")
	assert.Contains(t, record.Summary, "exit 3", "the summary names the gate that failed")
}

// §4.2 holds at most one object: a second failure overwrites the first rather
// than accumulating, and the surviving record belongs to the second unit.
func TestDoneGate_SecondFailureOverwritesTheFirst(t *testing.T) {
	dir := setupProjectWithGate(t, "echo boom; exit 7")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Task two","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, _, code := runTPEnv(t, dir, gateUnitEnv("implement", "t1", "implement"), "done", "t1", "task complete and verified fully")
	require.Equal(t, 4, code)
	_, _, code = runTPEnv(t, dir, gateUnitEnv("implement", "t2", "implement"), "done", "t2", "task complete and verified fully")
	require.Equal(t, 4, code)

	record := readGateLastFailure(t, dir)
	require.NotNil(t, record)
	assert.Equal(t, "t2", record.UnitID, "the second failure overwrites the first")
}

// `tp done --batch` runs the same gate for the same reason, so its failure is
// recorded on the same terms — the batch path does not exit through the
// pre-flock writer, which is exactly why it is asserted separately.
func TestDoneGate_BatchFailureUnderARunRecordsLastFailure(t *testing.T) {
	dir := setupProjectWithGate(t, `printf 'gate-tail-%s\n' marker; exit 7`)
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjson, []byte(`{"id":"a","reason":"A complete and verified"}`+"\n"), 0o600))

	_, _, code := runTPEnv(t, dir, gateUnitEnv("review-record", "3", "review"), "done", "--batch", ndjson)
	require.Equal(t, 4, code)

	record := readGateLastFailure(t, dir)
	require.NotNil(t, record, "the batch gate failure is recorded too")
	assert.Equal(t, engine.UnitReviewRecord, record.UnitKind)
	assert.Equal(t, "3", record.UnitID)
	assert.Equal(t, "review", record.Phase)
	assert.Equal(t, 7, record.ExitCode)
	assert.Contains(t, record.Summary, "gate-tail-marker", "summary carries the gate's own output")
}

// §4.2 names `tp done` as the record's second writer, and the low-level
// `tp close` is not it: the same failing gate under the same environment
// records nothing there, so the parameter that says so is load-bearing rather
// than decorative.
func TestCloseGate_FailureUnderARunRecordsNothing(t *testing.T) {
	dir := setupProjectWithGate(t, "echo boom; exit 7")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)
	_, stderr, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim failed: %s", stderr)

	_, stderr, code = runTPEnv(t, dir, gateUnitEnv("implement", "t1", "implement"), "close", "t1", "task complete and fully verified")
	require.Equal(t, 4, code, "the gate still fails the close: %s", stderr)

	assert.Nil(t, readGateLastFailure(t, dir), "tp close is not §4.2's writer")
}

// The record surfaces where §4.2 says it does: the next unit's brief reads it
// back, which is the whole point of writing it from tp done.
func TestDoneGate_RecordedFailureReachesTheNextBrief(t *testing.T) {
	dir := setupProjectWithGate(t, "echo boom; exit 7")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, _, code := runTPEnv(t, dir, gateUnitEnv("implement", "t1", "implement"), "done", "t1", "task complete and verified fully")
	require.Equal(t, 4, code)

	stdout, stderr, code := runTP(t, dir, "next", "--brief")
	require.Equal(t, 0, code, "next --brief failed: %s", stderr)

	var brief map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	failure, ok := brief["last_failure"].(map[string]any)
	require.True(t, ok, "the brief surfaces the record tp done wrote: %s", stdout)
	assert.Equal(t, "t1", failure["unit_id"])
	assert.Equal(t, float64(7), failure["exit_code"])
}

package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readRunStateRaw reads the run state file as generic JSON, so a test can tell
// a null from a zero — the distinction the whole two-write protocol rests on,
// and one a decode into RunState's pointers would still hide behind a nil.
func readRunStateRaw(t *testing.T, root, taskFile string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(RunStatePath(root, taskFile))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

// rawUnits returns the run state's unit rows as generic JSON objects.
func rawUnits(t *testing.T, raw map[string]any) []map[string]any {
	t.Helper()
	units, ok := raw["units"].([]any)
	require.True(t, ok, "units is a JSON array, not %T", raw["units"])
	out := make([]map[string]any, 0, len(units))
	for _, u := range units {
		row, isObject := u.(map[string]any)
		require.True(t, isObject, "each unit row is a JSON object")
		out = append(out, row)
	}
	return out
}

// startedRow is the row the driver appends before spawning a child. The result
// fields carry values deliberately: StartUnit must clear them.
func startedRow(seq, attempt int, id string) *RunUnitRow {
	code := 0
	seconds := 12.5
	spend := 3.25
	return &RunUnitRow{
		Seq:             seq,
		Kind:            UnitReviewRole,
		ID:              id,
		Attempt:         attempt,
		ExitCode:        &code,
		DurationSeconds: &seconds,
		SpendUSD:        &spend,
		LogPath:         "/runs/x/" + id + ".jsonl",
	}
}

// newTestRun creates a recorder for a fresh repository root, returning the root
// and the task file the run state is named after.
func newTestRun(t *testing.T) (root, taskFile string, rec *RunRecorder) {
	t.Helper()
	root = t.TempDir()
	taskFile = filepath.Join(root, "spec", "0.35.0.tasks.json")
	rec, err := NewRunRecorder(root, taskFile, NewULID(), PhaseImplement)
	require.NoError(t, err)
	return root, taskFile, rec
}

func TestRunStatePath_IsPerTaskFileUnderTheTPDir(t *testing.T) {
	root := t.TempDir()

	path := RunStatePath(root, filepath.Join(root, "spec", "0.35.0.tasks.json"))
	assert.Equal(t, filepath.Join(root, ".tp", "run-0.35.0.json"), path,
		"the run state is .tp/run-<base>.json, with <base> the task file minus .tasks.json")
	assert.True(t, filepath.IsAbs(path), "the path is absolute, like TP_RUN_DIR")

	other := RunStatePath(root, filepath.Join(root, "spec", "0.36.0.tasks.json"))
	assert.NotEqual(t, path, other, "two cycles in one repository never share a run state file")
}

func TestNewRunRecorder_WritesTheDocumentedShape(t *testing.T) {
	root, taskFile, _ := newTestRun(t)

	raw := readRunStateRaw(t, root, taskFile)
	for _, key := range []string{"run_id", "started_at", "phase", "stop_reason", "totals", "units"} {
		assert.Contains(t, raw, key, "§3.5's shape carries %s", key)
	}
	assert.Nil(t, raw["stop_reason"], "a run that has not stopped carries a null stop_reason")
	assert.Equal(t, PhaseImplement, raw["phase"])
	assert.Empty(t, raw["units"], "a run with no attempt yet holds no rows")
	assert.NotEmpty(t, raw["run_id"])

	_, err := time.Parse(time.RFC3339Nano, raw["started_at"].(string))
	assert.NoError(t, err, "started_at is a timestamp")

	totals := raw["totals"].(map[string]any)
	assert.Equal(t, 0.0, totals["units"])
	assert.Equal(t, 0.0, totals["spend_usd"])
	assert.Contains(t, totals, "wall_clock_seconds")
}

// Test 12: the row exists with a null exit_code before the child exits, and is
// updated in place after.
func TestRunRecorder_RowIsNullBeforeTheChildExitsAndUpdatedAfter(t *testing.T) {
	root, taskFile, rec := newTestRun(t)

	row := startedRow(1, 1, "architect")
	require.NoError(t, rec.StartUnit(row))
	assert.NotNil(t, row.ExitCode, "the caller's row is copied, not cleared in place")

	before := rawUnits(t, readRunStateRaw(t, root, taskFile))
	require.Len(t, before, 1)
	assert.Nil(t, before[0]["exit_code"], "the appended row's exit_code is null before the child exits")
	assert.Nil(t, before[0]["duration_seconds"], "so is its duration")
	assert.Nil(t, before[0]["spend_usd"], "so is its spend")
	assert.Equal(t, "architect", before[0]["id"])
	assert.Equal(t, string(UnitReviewRole), before[0]["kind"])
	assert.Equal(t, 1.0, before[0]["attempt"])
	assert.Equal(t, "/runs/x/architect.jsonl", before[0]["log_path"])

	spend := 0.42
	require.NoError(t, rec.FinishUnit(1, 3, 1500*time.Millisecond, &spend))

	after := rawUnits(t, readRunStateRaw(t, root, taskFile))
	require.Len(t, after, 1, "the row is updated in place, not appended a second time")
	assert.Equal(t, 3.0, after[0]["exit_code"], "the exit code the child produced")
	assert.InDelta(t, 1.5, after[0]["duration_seconds"], 0.001)
	assert.InDelta(t, 0.42, after[0]["spend_usd"], 0.0001)

	totals := readRunStateRaw(t, root, taskFile)["totals"].(map[string]any)
	assert.Equal(t, 1.0, totals["units"])
	assert.InDelta(t, 0.42, totals["spend_usd"], 0.0001)
}

func TestRunRecorder_UnmeteredUnitStaysNullAndAccruesNothing(t *testing.T) {
	root, taskFile, rec := newTestRun(t)

	require.NoError(t, rec.StartUnit(startedRow(1, 1, "implementer")))
	require.NoError(t, rec.FinishUnit(1, 0, time.Second, nil))

	raw := readRunStateRaw(t, root, taskFile)
	rows := rawUnits(t, raw)
	require.Len(t, rows, 1)
	assert.Equal(t, 0.0, rows[0]["exit_code"], "a zero exit code is recorded as zero, not left null")
	assert.Nil(t, rows[0]["spend_usd"], "a runner declaring no spend_key reports spend null")
	assert.Equal(t, 0.0, raw["totals"].(map[string]any)["spend_usd"],
		"an unmetered unit accrues nothing against the budget cap")
}


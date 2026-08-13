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


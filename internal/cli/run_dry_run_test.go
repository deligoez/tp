package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
)

// decodeDryRun parses a `tp run --dry-run` payload.
func decodeDryRun(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "dry-run payload: %s", stdout)
	return out
}

// unitPairs renders a next_units array as "<kind> <id>" strings, which is the
// shape a listing assertion reads at a glance.
func unitPairs(t *testing.T, units any) []string {
	t.Helper()
	rows, ok := units.([]any)
	require.True(t, ok, "next_units is an array: %#v", units)
	pairs := make([]string, 0, len(rows))
	for _, row := range rows {
		entry, isMap := row.(map[string]any)
		require.True(t, isMap, "every next_units entry is an object: %#v", row)
		pairs = append(pairs, fmt.Sprintf("%v %v", entry["kind"], entry["id"]))
	}
	return pairs
}

// §3.5 (tests 13 and 58): `tp run --dry-run` prints the units the driver would
// execute next and exits 0 without spawning anything or writing run state.
//
// Every negative half of that sentence is driven through the real path rather
// than asserted about a code path not taken: the runner seam is pinned to the
// fake runner with a live record directory, so a driver that spawned even one
// child would leave a record behind, and the run artifacts are checked for
// absence after the command has actually run.
func TestRunDryRun_ListsTheNextUnitsAndSpawnsNothing(t *testing.T) {
	dir := runProject(t)
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))

	stdout, stderr, code := runTPEnv(t, dir, []string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
		fakerunner.EnvDurable + "=1",
	}, "run", "--dry-run")
	require.Equal(t, 0, code, "tp run --dry-run exits 0: %s%s", stdout, stderr)

	out := decodeDryRun(t, stdout)
	assert.Equal(t, engine.PhaseImplement, out["phase"])
	assert.Equal(t, []string{"implement seed"}, unitPairs(t, out["next_units"]),
		"the unit the driver would execute next is listed")
	first := out["next_units"].([]any)[0].(map[string]any)
	assert.Equal(t, "tp next --brief", first["brief_command"],
		"each entry carries the brief its unit would run")

	// The runner was configured and would have recorded itself: nothing did.
	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	assert.Empty(t, recs, "--dry-run spawns nothing")

	assert.NoFileExists(t, filepath.Join(dir, ".tp", "run-spec.json"),
		"--dry-run writes no run state")
	assert.NoDirExists(t, filepath.Join(dir, ".tp", "runs"),
		"--dry-run creates no run directory")

	// The cycle itself is untouched, which is the observable a spawned
	// implement unit would have changed: TP_FAKE_RUNNER_DURABLE=1 closes the
	// task it is given.
	showOut, stderr, code := runTP(t, dir, "show", "seed")
	require.Equal(t, 0, code, "show: %s", stderr)
	assert.Contains(t, showOut, `"status": "open"`, "--dry-run advances nothing")
}

// §3.5 (test 58): the dry-run sub-mode takes no run lock.
//
// The lock is held for the whole assertion, and the control arm in the same
// fixture is a real `tp run` refused with exit 4 — so a dry run that exits 0
// here is evidence the lock was not taken, rather than evidence the lock was
// never held.
func TestRunDryRun_TakesNoRunLock(t *testing.T) {
	dir := runProject(t)
	taskFile := filepath.Join(dir, "spec.tasks.json")

	acquired := make(chan struct{})
	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		defer close(held)
		lockErr := engine.WithRunLock(taskFile, func() error {
			close(acquired)
			<-release
			return nil
		})
		assert.NoError(t, lockErr)
	}()
	<-acquired
	defer func() {
		close(release)
		<-held
	}()

	_, _, code := runTP(t, dir, "run")
	require.Equal(t, 4, code, "control: a driving run does take the run lock")

	stdout, stderr, code := runTP(t, dir, "run", "--dry-run")
	assert.Equal(t, 0, code, "--dry-run takes no run lock: %s%s", stdout, stderr)
	assert.Equal(t, []string{"implement seed"},
		unitPairs(t, decodeDryRun(t, stdout)["next_units"]),
		"it still reports, rather than reporting nothing because it was blocked")
}


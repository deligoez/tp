package engine

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// oneOpenTask is the smallest cycle that owes an implement unit, so a retry
// test observes one unit's attempts and nothing else.
const oneOpenTask = `{"spec":"s.md","tasks":[
 {"id":"alpha","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`

// readRunStateFile reads the run state a driver left behind, which is where the
// per-attempt rows §3.5 documents can be counted.
func readRunStateFile(t *testing.T, root, taskFile string) RunState {
	t.Helper()
	data, err := os.ReadFile(RunStatePath(root, taskFile))
	require.NoError(t, err)
	var st RunState
	require.NoError(t, json.Unmarshal(data, &st))
	return st
}

// §3.4: a unit is attempted `1 + run_max_unit_retries` times, and exhausting
// them stops the run with unit-failure.
//
// The exact count is the whole assertion, at three budgets rather than one: an
// off-by-one in either direction passes a test that only asks whether a retry
// happened, and 0 retries is the arm that proves the budget is read at all
// rather than a constant being applied.
func TestRunDriver_AttemptsAUnitItsBudgetedNumberOfTimes(t *testing.T) {
	cases := []struct {
		retries  int
		attempts int
	}{
		{retries: 0, attempts: 1},
		{retries: 1, attempts: 2},
		{retries: 2, attempts: 3},
	}
	for _, tc := range cases {
		t.Run("retries="+strconv.Itoa(tc.retries), func(t *testing.T) {
			root, spec, taskFile, records := seamProject(t, oneOpenTask)

			// The fake exits 0 and writes nothing: every attempt is a
			// failed attempt, so the run spends the whole budget.
			wf := driverWorkflow()
			wf.RunMaxUnitRetries = tc.retries
			res := driveOnce(t, root, spec, taskFile, wf)

			assert.Equal(t, StopUnitFailure, res.StopReason,
				"a unit that exhausts its attempts stops the run")

			ids := spawnedIDs(invocations(t, records))
			assert.Len(t, ids, tc.attempts,
				"a unit is attempted 1 + run_max_unit_retries times, exactly")
			for _, id := range ids {
				assert.Equal(t, "alpha", id, "every attempt is at the same unit")
			}

			// totals.units counts attempts rather than units, so the cap,
			// the totals and --status's units-done read the same number.
			assert.Equal(t, tc.attempts, res.Units, "totals.units counts attempts")

			st := readRunStateFile(t, root, taskFile)
			require.Len(t, st.Units, tc.attempts, "one run-state row per attempt")
			seqs := map[int]bool{}
			logs := map[string]bool{}
			for i, row := range st.Units {
				assert.Equal(t, i+1, row.Attempt, "the attempt counter numbers the attempts from 1")
				assert.False(t, seqs[row.Seq], "every attempt takes a fresh seq")
				assert.False(t, logs[row.LogPath], "every attempt takes a fresh log path")
				seqs[row.Seq] = true
				logs[row.LogPath] = true
			}
		})
	}
}

// A retry is a real second chance: a unit that fails once and succeeds on its
// next attempt leaves the run to continue, rather than the first failure
// standing.
func TestRunDriver_AUnitThatSucceedsOnItsRetryLetsTheRunContinue(t *testing.T) {
	root, spec, taskFile, records := seamProject(t, oneOpenTask)
	recordRounds(t, spec, 0, 2, true)

	// The first invocation exits 0 having written nothing — a failed attempt
	// — and the second performs the durable write.
	t.Setenv(fakerunner.EnvDurable, "0,1")

	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1
	res := driveOnce(t, root, spec, taskFile, wf)

	assert.Equal(t, StopConverged, res.StopReason, "the retry finished the unit and the cycle released")
	assert.Len(t, spawnedIDs(invocations(t, records)), 2, "one failed attempt and one that finished it")
}


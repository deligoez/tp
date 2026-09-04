package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
)

// decodeStatus parses a `tp run --status` payload.
func decodeStatus(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "status payload: %s", stdout)
	return out
}

// seamEnv is the environment a real run through the test seam needs: the fake
// runner as the runner, and a fresh directory for it to record itself in.
func seamEnv(t *testing.T) []string {
	t.Helper()
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))
	return []string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
	}
}

// writeRunState hand-writes a run state file, which is how the two states a
// finished run cannot produce — a driver still working, and one that died
// before writing a stop reason — are reached at all.
func writeRunState(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "run-spec.json"), []byte(body), 0o600))
}

// crashedRunState is a run whose driver never wrote a stop reason: rows
// present, stop_reason null. It is what a killed driver leaves behind.
const crashedRunState = `{"run_id":"01HZZZZZZZZZZZZZZZZZZZZZZZ","started_at":"2026-08-30T00:00:00Z",
 "phase":"implement","stop_reason":null,
 "totals":{"units":1,"wall_clock_seconds":12,"spend_usd":0},
 "units":[{"seq":1,"kind":"implement","id":"seed","attempt":1,"exit_code":null,
           "duration_seconds":null,"spend_usd":null,"log_path":"/tmp/run/1-implement-seed.jsonl"}]}`

// §3.5: `tp run --status` exits 3 when no run state exists for the resolved
// task file (test 58). This is the boundary a test that only runs after a
// successful run never reaches, so it gets its own fixture: a cycle that has
// never been driven.
func TestRunStatus_ExitsThreeWhenNoRunStateExists(t *testing.T) {
	t.Parallel()
	dir := runProject(t)

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	assert.Equal(t, 3, code, "no run state is exit 3, not a failure: %s%s", stdout, stderr)
	assert.NoFileExists(t, filepath.Join(dir, ".tp", "run-spec.json"),
		"--status reports on run state; it never creates it")
	assert.Contains(t, stderr, "run state", "the error names what is missing")
}

// §3.5: a run that has ended reports its phase, its units done, its accrual
// against each cap, the last unit's exit code and log path, and its stop
// reason, with run_state stopped.
func TestRunStatus_ReportsAStoppedRun(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	env := append(seamEnv(t), fakerunner.EnvDurable+"=1")

	runOut, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "the run stopped without converging, which section 3.4 exits 4 on: %s", stderr)
	driven := decodeStatus(t, runOut)

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status exits 0 when it can report: %s", stderr)
	st := decodeStatus(t, stdout)

	assert.Equal(t, "stopped", st["run_state"], "a stop reason is present, so the run is stopped")
	assert.Equal(t, driven["stop_reason"], st["stop_reason"], "--status reports the run's own stop reason")
	assert.Equal(t, driven["phase"], st["phase"])
	assert.Equal(t, driven["run_id"], st["run_id"])
	assert.NotEmpty(t, st["started_at"])
	assert.Equal(t, driven["units"], st["units_done"], "units-done is the run's own unit count")

	caps, ok := st["caps"].(map[string]any)
	require.True(t, ok, "the accrual is reported against each cap")
	assert.Equal(t, float64(engine.RunMaxUnitsDefault), caps["max_units"])
	assert.Equal(t, float64(engine.RunMaxWallClockSecondsDefault), caps["max_wall_clock_seconds"])
	assert.Equal(t, engine.RunMaxBudgetUSDDefault, caps["max_budget_usd"])
	assert.Contains(t, st, "wall_clock_seconds")
	assert.Contains(t, st, "spend_usd")

	last, ok := st["last_unit"].(map[string]any)
	require.True(t, ok, "the last unit is reported")
	assert.Equal(t, float64(0), last["exit_code"], "the last unit's exit code, from a real child")
	assert.NotEmpty(t, last["log_path"], "the last unit's log path")
	assert.NotEmpty(t, last["kind"])
}

// §3.4/§3.5 test 33: run_max_units, totals.units and --status's units-done all
// count ATTEMPTS. The fixture is deliberately one where the two numbers differ
// — one unit attempted twice — because a fixture where they are equal passes
// whichever the implementation counts.
func TestRunStatus_UnitsDoneCountsAttemptsNotDistinctUnits(t *testing.T) {
	t.Parallel()
	dir := runProject(t)

	// No EnvDurable: every attempt exits 0 having written nothing, so the one
	// implement unit spends its whole attempt budget (1 + the default 1 retry).
	_, stderr, code := runTPEnv(t, dir, seamEnv(t), "run")
	require.Equal(t, 4, code, "the exhausted unit stops the run non-converged, which exits 4: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	st := decodeStatus(t, stdout)

	assert.Equal(t, float64(2), st["units_done"],
		"one unit attempted twice is two units done, not one")

	// The distinctness the assertion above rests on: both rows are the same unit.
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "run-spec.json"))
	require.NoError(t, err)
	var raw engine.RunState
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, raw.Units, 2, "two attempt rows")
	assert.Equal(t, raw.Units[0].ID, raw.Units[1].ID, "at one and the same unit")
	assert.Equal(t, 2, raw.Totals.Units, "totals.units reads the same number")

	// A stop outside the audit phase carries none of the divergence signals.
	assert.NotContains(t, st, "role_streaks", "the signals travel on an audit-phase stop only")
	assert.NotContains(t, st, "spec_coverage_clean_rounds")
	assert.Equal(t, engine.PhaseImplement, st["phase"])
}

// §3.5: run_state is stopped once stop_reason is set, otherwise in_flight when
// the run lock is held and crashed when it is not — the lock is the only
// evidence separating the last two (test 35).
//
// The three arms run against ONE run state file in ONE directory, so the only
// thing that varies between them is the lock. The final arm re-reads with the
// lock file present but unheld, which is what discriminates "the lock is held"
// from "a lock file exists".
func TestRunStatus_ReportsInFlightCrashedAndStopped(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	writeRunState(t, dir, crashedRunState)
	taskFile := filepath.Join(dir, "spec.tasks.json")

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	assert.Equal(t, "crashed", decodeStatus(t, stdout)["run_state"],
		"no stop reason and no lock: the driver died")

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

	stdout, stderr, code = runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status takes no run lock, so it reports while one is held: %s", stderr)
	assert.Equal(t, "in_flight", decodeStatus(t, stdout)["run_state"],
		"no stop reason and the run lock held: a driver is still working")

	close(release)
	<-held

	stdout, stderr, code = runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	assert.Equal(t, "crashed", decodeStatus(t, stdout)["run_state"],
		"the lock FILE outlives the lock; only the lock separates crashed from in_flight")
	assert.FileExists(t, filepath.Join(dir, ".tp", "locks", "run-spec.lock"),
		"and that file is still there, so the arm above is not passing on its absence")
}

// §1/§3.5: the divergence signal — spec_coverage_clean_rounds, role_streaks and
// divergence — travels in `tp run --status` on any audit-phase stop, verbatim
// from `tp audit --status` (test 14).
func TestRunStatus_CarriesTheDivergenceSignalsOnAnAuditPhaseStop(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	env := append(seamEnv(t), fakerunner.EnvDurable+"=1")

	runOut, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "the run stopped without converging, which section 3.4 exits 4 on: %s", stderr)
	require.Equal(t, engine.PhaseAudit, decodeStatus(t, runOut)["phase"],
		"the fixture must stop in the audit phase for this test to discriminate")

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	st := decodeStatus(t, stdout)

	auditOut, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "tp audit --status: %s", stderr)
	audit := decodeStatus(t, auditOut)

	require.Contains(t, st, "role_streaks", "the audit-phase stop carries the signals")
	require.Contains(t, st, "spec_coverage_clean_rounds")
	assert.Equal(t, audit["role_streaks"], st["role_streaks"], "verbatim from tp audit --status")
	assert.Equal(t, audit["spec_coverage_clean_rounds"], st["spec_coverage_clean_rounds"])
	assert.Equal(t, audit["divergence"], st["divergence"],
		"divergence is carried when audit carries it and omitted when audit omits it")
}

// Section 7 test 68: under --compact the run surface keeps stop_reason and the
// cap totals and strips per-unit rows and log paths.
//
// The control arm reads the SAME run without --compact and requires the row and
// its log path to be there. Without it the compact assertions would pass on a
// fixture that never carried the case — a run that spawned nothing has no row
// to strip, and the test would be measuring its own fixture.
func TestRunStatus_CompactStripsUnitRowsAndLogPaths(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	env := append(seamEnv(t), fakerunner.EnvDurable+"=1")

	runOut, stderr, code := runTPEnv(t, dir, env, "run", "--compact")
	require.Equal(t, 4, code, "the run stopped without converging, which section 3.4 exits 4 on: %s", stderr)
	driven := decodeStatus(t, runOut)
	assert.NotNil(t, driven["stop_reason"], "the driving surface keeps its stop reason under --compact")
	assert.NotContains(t, runOut, "log_path",
		"tp run reports a unit COUNT, so its own payload carries no row to strip")

	full, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	fullSt := decodeStatus(t, full)
	last, ok := fullSt["last_unit"].(map[string]any)
	require.True(t, ok, "the control arm must carry a per-unit row, or --compact strips nothing")
	require.NotEmpty(t, last["log_path"], "and a log path, for the same reason")

	out, stderr, code := runTP(t, dir, "run", "--status", "--compact")
	require.Equal(t, 0, code, "--status --compact: %s", stderr)
	st := decodeStatus(t, out)

	// Kept: stop_reason and the cap totals.
	assert.Equal(t, fullSt["stop_reason"], st["stop_reason"], "stop_reason survives --compact")
	assert.Equal(t, fullSt["caps"], st["caps"], "the caps survive whole")
	assert.Equal(t, fullSt["units_done"], st["units_done"], "and so does the accrual against them")
	assert.Contains(t, st, "wall_clock_seconds")
	assert.Contains(t, st, "spend_usd")

	// Stripped: the per-unit row, and with it every log path.
	assert.NotContains(t, st, "last_unit", "the per-unit row is stripped")
	assert.NotContains(t, out, "log_path", "and no log path survives anywhere in the payload")
}

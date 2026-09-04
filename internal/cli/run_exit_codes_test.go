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

// §3.4 test 54, driven through the real command: `tp run` exits 0 on
// `converged` and 4 on every other stop reason, and a usage error raised before
// the loop starts exits 2.
//
// Each case reaches its stop reason by arranging the condition §3.4 names and
// letting the loop find it — a releasable cycle, a phase the oracle offers no
// unit for, a unit that never writes, a cap set below what the cycle needs, an
// escalation record — rather than by asserting over a hand-built result. The
// exit code and the recorded stop reason are checked together, so a run that
// exits 4 for the wrong reason fails as loudly as one that exits 0.

// runExitProject seeds a project whose task file the caller supplies verbatim.
// The states these tests need — a task already done, a task depending on
// nothing that exists — are ones tp's own write commands deliberately refuse to
// produce, so the fixture writes the file directly after tp init has laid down
// the rest of the project.
func runExitProject(t *testing.T, tasksJSON string) (dir string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Exit Spec\n## 1. Setup\n### 1.1 Seed\nSeed the project.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(tasksJSON), 0o600))
	return dir
}

const exitDoneTask = `{"spec":"spec.md","tasks":[{"id":"seed","title":"Seed","status":"done",` +
	`"depends_on":[],"estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"]}]}`

const exitBlockedTask = `{"spec":"spec.md","tasks":[{"id":"seed","title":"Seed","status":"open",` +
	`"depends_on":["missing"],"estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"]}]}`

// recordCleanAuditRounds stamps n clean audit rounds at the spec's current
// hash, which is what makes the cycle releasable (§4.1).
func recordCleanAuditRounds(t *testing.T, specPath string, n int) {
	t.Helper()
	hash, err := engine.SpecHash(specPath)
	require.NoError(t, err)
	st, err := engine.EnsureReviewState(specPath)
	require.NoError(t, err)
	for i := range n {
		st.AuditRounds = append(st.AuditRounds, engine.ReviewRound{Round: i + 1, Clean: true, SpecHash: hash})
	}
	require.NoError(t, engine.SaveReviewState(specPath, st))
}

// seam pins TP_RUNNER_SEAM to a freshly built fake runner and returns the
// environment entries plus the directory it records its invocations in. Every
// case here pins it, including the ones that must spawn nothing: a run that
// wrongly spawns a child then leaves a record to prove it, instead of silently
// executing whatever `claude` happens to be on the test host's PATH.
func seam(t *testing.T, extra ...string) (env []string, records string) {
	t.Helper()
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records = filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))
	return append([]string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
	}, extra...), records
}

// stopReasonOf decodes the payload `tp run` printed and returns its stop reason.
func stopReasonOf(t *testing.T, stdout string) string {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "tp run prints its payload whatever it exits with")
	reason, _ := out["stop_reason"].(string)
	return reason
}

// Converged is the one stop reason `tp run` exits 0 on, and the only one the
// loop reaches by agreement rather than by exhaustion. Nothing is spawned: the
// releasable check is the loop's first step.
func TestRunExitCodes_ConvergedExitsZero(t *testing.T) {
	t.Parallel()
	dir := runExitProject(t, exitDoneTask)
	recordCleanAuditRounds(t, filepath.Join(dir, "spec.md"), 2)
	env, records := seam(t)

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 0, code, "a releasable cycle exits 0: %s", stderr)
	assert.Equal(t, string(engine.StopConverged), stopReasonOf(t, stdout))

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	assert.Empty(t, recs, "a releasable cycle spawns nothing")
}

// no-units: the oracle reports a phase it can offer no unit for — here an open
// task whose dependency does not exist, which is §4.6's escalate blocker — and
// the run stops without spawning anything. Not converged, so exit 4.
func TestRunExitCodes_NoUnitsExitsFour(t *testing.T) {
	t.Parallel()
	dir := runExitProject(t, exitBlockedTask)
	env, records := seam(t)

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "a run that stopped without converging is a state error: %s", stderr)
	assert.Equal(t, string(engine.StopNoUnits), stopReasonOf(t, stdout))

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	assert.Empty(t, recs, "a phase with no unit spawns nothing")
}

// unit-failure: the child exits 0 but leaves no durable write, so every attempt
// fails and the run stops on the exhausted unit. Exit 4.
func TestRunExitCodes_UnitFailureExitsFour(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	env, records := seam(t)

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "an exhausted unit is a state error: %s", stderr)
	assert.Equal(t, string(engine.StopUnitFailure), stopReasonOf(t, stdout))

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	assert.Len(t, recs, 2, "1 + the default run_max_unit_retries attempts")
}

// cap-units: a cap trips on a cycle that is otherwise making progress. A cap is
// never an acceptance (§3.4), so the run reports 4 even though every unit it
// ran succeeded.
func TestRunExitCodes_CapUnitsExitsFour(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "run_max_units=1")
	require.Equal(t, 0, code, "lowering a cap is accepted: %s", stderr)
	env, records := seam(t, fakerunner.EnvDurable+"=1")

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "a cap stop is a state error: %s", stderr)
	assert.Equal(t, string(engine.StopCapUnits), stopReasonOf(t, stdout))

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	assert.Len(t, recs, 1, "the cap stopped the run after its one permitted attempt")

	showOut, stderr, code := runTP(t, dir, "show", "seed")
	require.Equal(t, 0, code, "show: %s", stderr)
	assert.Contains(t, showOut, `"status": "done"`, "the unit that ran did succeed — the cap, not a failure, ended the run")
}

// escalation: a unit asked for a decision only a user can make. It is a normal,
// expected outcome and still exits 4, because the run needs a human.
func TestRunExitCodes_EscalationExitsFour(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	env, _ := seam(t, fakerunner.EnvEscalate+"=1")

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "an escalation stop is a state error: %s", stderr)
	assert.Equal(t, string(engine.StopEscalation), stopReasonOf(t, stdout))
}

// A usage error raised before the loop starts exits 2, and the negative half
// matters as much as the code: nothing was spawned and no run state was
// written, so the 2 really is "tp did not run the request" rather than a run
// that started and then failed.
func TestRunExitCodes_UsageErrorBeforeTheLoopExitsTwo(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"too many arguments": {"run", "spec.md", "extra"},
		"unknown flag":       {"run", "--nope"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := runProject(t)
			env, records := seam(t)

			_, stderr, code := runTPEnv(t, dir, env, args...)
			require.Equal(t, 2, code, "a usage error exits 2: %s", stderr)

			recs, err := fakerunner.Records(records)
			require.NoError(t, err)
			assert.Empty(t, recs, "the refusal precedes every spawn")
			assert.NoFileExists(t, filepath.Join(dir, ".tp", "run-spec.json"),
				"the refusal precedes the run state the driver writes")
			assert.NoDirExists(t, filepath.Join(dir, ".tp", "runs"),
				"the refusal precedes the run directory the driver creates")
		})
	}
}

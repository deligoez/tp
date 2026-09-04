package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
)

// runTPEnv runs tp with extra environment entries, which the loop's tests need
// in order to pin TP_RUNNER_SEAM and the fake runner's own knobs.
func runTPEnv(t *testing.T, dir string, extra []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, append([]string{"--json"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "NO_COLOR=1", "TP_HC=0"), extra...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running tp: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// runProject seeds a one-task cycle the driver can drive end to end.
func runProject(t *testing.T) (dir string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Driver Spec\n## 1. Setup\n### 1.1 Seed\nSeed the project.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "add",
		`{"id":"seed","title":"Seed","estimate_minutes":5,"acceptance":"Seeded.","source_sections":["### 1.1 Seed"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	return dir
}

// tp run drives the whole cycle from the command line: it spawns the implement
// unit the oracle named, re-reads the cycle from disk, crosses into the audit
// phase as the task closes, fans the audit role panel out in one iteration, and
// then takes the round's record unit (§4.1) rather than stopping on no-units.
// The fake runner writes no record kind's durable artifact, so that unit
// exhausts its attempts — which is where this end-to-end drive stops.
func TestRunCommand_DrivesTheCycleThroughTheSeam(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))

	stdout, stderr, code := runTPEnv(t, dir, []string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
		fakerunner.EnvDurable + "=1",
	}, "run")
	// §3.4: the run really did drive the whole cycle, but it stopped on the
	// record unit's exhausted attempts rather than converged, and every
	// non-converged stop exits 4.
	require.Equal(t, 4, code, "tp run: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, engine.PhaseAudit, out["phase"], "the loop crossed from implement into audit")
	assert.Equal(t, string(engine.StopUnitFailure), out["stop_reason"])
	assert.NotEmpty(t, out["run_id"])

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	require.Len(t, recs, 6,
		"one implement unit, the three-role audit panel, then the round's record unit twice")
	assert.Equal(t, string(engine.UnitImplement), recs[0].Env[engine.EnvUnitKind])
	assert.Equal(t, "seed", recs[0].Env[engine.EnvUnitID])
	for _, rec := range recs[1:4] {
		assert.Equal(t, string(engine.UnitAuditRole), rec.Env[engine.EnvUnitKind])
	}
	for _, rec := range recs[4:] {
		assert.Equal(t, string(engine.UnitAuditRecord), rec.Env[engine.EnvUnitKind],
			"the collected round owes its record unit (§4.1), retried once by the default budget")
		assert.Equal(t, "1", rec.Env[engine.EnvUnitID], "a record unit's id is its round number")
	}
	// Every child carries the identity §3.1.1 fixes, from a real spawn.
	assert.Equal(t, "1", recs[0].Env[engine.EnvUnattended])
	assert.Equal(t, "spec.tasks.json", recs[0].Env[engine.EnvTaskFile],
		"the child works the target the driver resolved, read from the root it is spawned in")
	assert.Equal(t, filepath.Join(dir, ".tp", "runs", recs[0].Env[engine.EnvRunID]), recs[0].Env[engine.EnvRunDir])

	// The task the implement unit closed is done on disk, and the run state
	// carries one row per attempt.
	showOut, stderr, code := runTP(t, dir, "show", "seed")
	require.Equal(t, 0, code, "show: %s", stderr)
	assert.Contains(t, showOut, `"status": "done"`)

	stateData, err := os.ReadFile(filepath.Join(dir, ".tp", "run-spec.json"))
	require.NoError(t, err)
	var state map[string]any
	require.NoError(t, json.Unmarshal(stateData, &state))
	assert.Equal(t, string(engine.StopUnitFailure), state["stop_reason"])
	assert.Len(t, state["units"], 6)
}

// The run-scoped lock is held for the whole run, so a second tp run over the
// same task file is refused with exit 4 rather than driving the same cycle
// twice (§3.1.1, test 9).
func TestRunCommand_SecondRunOverTheSameCycleExitsFour(t *testing.T) {
	t.Parallel()
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

	stdout, stderr, code := runTP(t, dir, "run")
	assert.Equal(t, 4, code, "a second tp run over a driven cycle is a state error")
	assert.Contains(t, stdout+stderr, "run-spec.lock", "the refusal names the run-scoped lock")
}

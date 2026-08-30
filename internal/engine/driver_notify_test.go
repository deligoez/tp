package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// notifyProbe writes the script §5.2's notify_cmd is observed through, and
// returns it beside the file it records itself in.
//
// It is a real executable rather than a double because every property §5.2
// fixes is a property of the process boundary: which variables crossed it, what
// argv arrived, and what code came back. The recording path is baked into the
// script rather than passed as an argument, so the argv a test asserts on is
// exactly what splitting the configured command produced and nothing the
// harness added.
//
// ${TP_ESCALATION_PATH-<unset>} is the one shell subtlety that earns its place:
// it distinguishes an absent variable from an empty one, which is the
// difference §5.2 draws between an escalation stop and every other stop.
func notifyProbe(t *testing.T, exitCode int) (script, out string) {
	t.Helper()
	dir := t.TempDir()
	out = filepath.Join(dir, "notify.txt")
	script = filepath.Join(dir, "notify.sh")
	body := "#!/bin/sh\n" +
		"{\n" +
		"printf 'reason=%s\\n' \"$TP_STOP_REASON\"\n" +
		"printf 'run_id=%s\\n' \"$TP_RUN_ID\"\n" +
		"printf 'escalation=%s\\n' \"${TP_ESCALATION_PATH-<unset>}\"\n" +
		"for a in \"$@\"; do printf 'arg=%s\\n' \"$a\"; done\n" +
		"} > \"" + out + "\"\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o700)) //nolint:gosec // a test fixture that has to be executable
	return script, out
}

// notifyLines returns what the probe recorded, and nil when it never ran at
// all. The absent file is the observation a "does not fire" assertion needs.
func notifyLines(t *testing.T, out string) []string {
	t.Helper()
	data, err := os.ReadFile(out)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// Test 42, at the one sink every stop passes through: notify_cmd fires on every
// non-converged stop and on no converged one, and firing it never changes the
// reason the run recorded.
//
// The table is driven off the stop vocabulary itself rather than off a list
// this test keeps, so a tenth reason cannot be added without being answered
// here — an unplaced reason is exercised the moment it exists.
func TestDriverStop_NotifyFiresOnEveryNonConvergedStop(t *testing.T) {
	for _, reason := range stopPrecedence {
		t.Run(string(reason), func(t *testing.T) {
			root := t.TempDir()
			taskFile := filepath.Join(root, "spec.tasks.json")
			script, out := notifyProbe(t, 0)

			runID := NewULID()
			rec, err := NewRunRecorder(root, taskFile, runID, PhaseImplement)
			require.NoError(t, err)

			wf := driverWorkflow()
			wf.NotifyCmd = script
			d := &driver{
				opts:  &DriverOptions{Root: root, TaskFile: taskFile, Workflow: *wf},
				runID: runID,
				rec:   rec,
			}
			result := DriverResult{RunID: runID}
			if reason == StopEscalation {
				result.EscalationPath = filepath.Join(root, "1-escalation.json")
			}
			got := d.stop(&result, reason)

			assert.Equal(t, reason, got.StopReason,
				"a notify_cmd never changes the reason the run stopped for")
			if reason == StopConverged {
				assert.Nil(t, notifyLines(t, out),
					"convergence is the run's own agreed ending, not a report to a human")
				return
			}
			lines := notifyLines(t, out)
			require.NotEmpty(t, lines, "every non-converged stop notifies")
			assert.Equal(t, "reason="+string(reason), lines[0],
				"the notification carries the reason the run stopped for")
		})
	}
}

// §5.2: on an escalation the command carries all three variables, and
// TP_ESCALATION_PATH is the record the run stopped on.
func TestRunDriver_NotifyCarriesTheEscalationPath(t *testing.T) {
	root, spec, taskFile, _ := seamProject(t, oneOpenTask)
	t.Setenv(fakerunner.EnvExits, "2")
	t.Setenv(fakerunner.EnvEscalate, "1")
	script, out := notifyProbe(t, 0)

	wf := driverWorkflow()
	wf.NotifyCmd = script
	res := driveOnce(t, root, spec, taskFile, wf)

	require.Equal(t, StopEscalation, res.StopReason)
	require.NotEmpty(t, res.EscalationPath)
	assert.Equal(t, []string{
		"reason=escalation",
		"run_id=" + res.RunID,
		"escalation=" + res.EscalationPath,
	}, notifyLines(t, out))
}

// §5.2: TP_ESCALATION_PATH exists on an escalation and nowhere else. A stale
// value inherited from the driver's own environment is removed rather than
// carried, so the variable's presence is the fact it claims to be.
func TestRunDriver_NotifyOmitsTheEscalationPathOnEveryOtherStop(t *testing.T) {
	root, spec, taskFile, _ := seamProject(t, oneOpenTask)
	t.Setenv(fakerunner.EnvExits, "1")
	t.Setenv("TP_ESCALATION_PATH", "/a/path/from/the/parent/environment")
	script, out := notifyProbe(t, 0)

	wf := driverWorkflow()
	wf.NotifyCmd = script
	res := driveOnce(t, root, spec, taskFile, wf)

	require.Equal(t, StopUnitFailure, res.StopReason)
	assert.Equal(t, []string{
		"reason=unit-failure",
		"run_id=" + res.RunID,
		"escalation=<unset>",
	}, notifyLines(t, out))
}

// §5.2: the command is exec'd without a shell and split on whitespace. The
// configured string carries what a shell would act on — a command separator, a
// variable reference, a run of spaces — and every one of them arrives at the
// child as literal text in its own argument.
func TestRunDriver_NotifyIsExecdWithoutAShell(t *testing.T) {
	root, spec, taskFile, _ := seamProject(t, oneOpenTask)
	t.Setenv(fakerunner.EnvExits, "1")
	script, out := notifyProbe(t, 0)

	wf := driverWorkflow()
	wf.NotifyCmd = script + " one;two $HOME   spaced"
	res := driveOnce(t, root, spec, taskFile, wf)

	require.Equal(t, StopUnitFailure, res.StopReason)
	lines := notifyLines(t, out)
	require.Len(t, lines, 6, "the three fixed lines, then one per argument")
	assert.Equal(t, []string{"arg=one;two", "arg=$HOME", "arg=spaced"}, lines[3:],
		"a shell would have split on the semicolon and expanded $HOME")
}


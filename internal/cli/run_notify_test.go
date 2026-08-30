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

// setLocalNotifyCmd configures notify_cmd the one way §7 allows: in
// .tp/local.json, merged into whatever the project already keeps there so the
// active pointer survives.
func setLocalNotifyCmd(t *testing.T, dir, command string) {
	t.Helper()
	path := filepath.Join(dir, ".tp", "local.json")
	local := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		require.NoError(t, json.Unmarshal(data, &local))
	}
	local["notify_cmd"] = command

	data, err := json.Marshal(local)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// notifyProbeScript writes a script that records the stop it was told about and
// exits with the given code, and returns it beside the file it records in.
func notifyProbeScript(t *testing.T, exitCode string) (script, out string) {
	t.Helper()
	dir := t.TempDir()
	out = filepath.Join(dir, "notify.txt")
	script = filepath.Join(dir, "notify.sh")
	body := "#!/bin/sh\n" +
		"printf '%s %s\\n' \"$TP_STOP_REASON\" \"$TP_RUN_ID\" > \"" + out + "\"\n" +
		"exit " + exitCode + "\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o700)) //nolint:gosec // a test fixture that has to be executable
	return script, out
}

// §5.2 end to end: a run that stops for anything but convergence invokes the
// operator's notify_cmd, tells it which run stopped and why, and reports what
// it did — while the stop reason and the exit code stay the run's own.
func TestRunCommand_ReportsWhatNotifyCmdDid(t *testing.T) {
	dir := runProject(t)
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))

	script, out := notifyProbeScript(t, "7")
	setLocalNotifyCmd(t, dir, script)

	stdout, stderr, code := runTPEnv(t, dir, []string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
	}, "run")
	// The unit never performs its durable write, so it spends both attempts
	// and the run stops with unit-failure — exit 4, whatever notify_cmd did.
	require.Equal(t, 4, code, "tp run: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.Equal(t, string(engine.StopUnitFailure), payload["stop_reason"],
		"a notify_cmd that exits 7 never changes the reason the run stopped for")

	notify, ok := payload["notify"].(map[string]any)
	require.True(t, ok, "a configured notify_cmd is reported beside the stop it announced")
	assert.Equal(t, script, notify["cmd"])
	assert.Equal(t, float64(7), notify["exit_code"], "the command's own exit code is reported")
	assert.Nil(t, notify["error"])

	recorded, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, string(engine.StopUnitFailure)+" "+payload["run_id"].(string)+"\n", string(recorded),
		"the command was told which run stopped and why")
}

// With no notify_cmd configured the payload carries no notification at all,
// rather than an empty report of a command that was never run.
func TestRunCommand_NoNotifyCmdReportsNothing(t *testing.T) {
	dir := runProject(t)
	bin, err := fakerunner.Build(t.TempDir())
	require.NoError(t, err)
	records := filepath.Join(t.TempDir(), "records")
	require.NoError(t, os.MkdirAll(records, 0o750))

	stdout, stderr, code := runTPEnv(t, dir, []string{
		engine.EnvRunnerSeam + "=" + bin,
		fakerunner.EnvDir + "=" + records,
	}, "run")
	require.Equal(t, 4, code, "tp run: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	assert.NotContains(t, payload, "notify")
}

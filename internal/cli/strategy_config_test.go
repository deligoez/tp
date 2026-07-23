package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTPHC runs the tp binary with an explicit TP_HC override ("1"/"0"), so a
// test can exercise the auto commit_strategy resolving to hc or builtin
// regardless of whether hc is installed on the host.
func runTPHC(t *testing.T, dir, hc string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	full := append([]string{"--json"}, args...)
	cmd := exec.Command(binaryPath, full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TP_HC="+hc)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	stdout = string(out)
	stderr = errBuf.String()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run tp: %v", err)
	}
	return stdout, stderr, code
}

func writeStrategyProject(t *testing.T, workflow string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[],"workflow":`+workflow+`}`), 0o600))
	return dir
}

func TestConfigCommitStrategyEffective(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	out, _, code := runTPHC(t, dir, "1", "config")
	require.Equal(t, 0, code)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &cfg))
	assert.Equal(t, "hc", cfg["commit_strategy_effective"], "auto with hc present is effective hc")

	out, _, code = runTPHC(t, dir, "0", "config")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(out), &cfg))
	assert.Equal(t, "builtin", cfg["commit_strategy_effective"], "auto with hc absent is effective builtin")
}

func TestConfigResolvedCommitStrategyDefault(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	out, _, code := runTPHC(t, dir, "0", "config", "--resolved")
	require.Equal(t, 0, code)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &cfg))
	wf := cfg["workflow"].(map[string]any)
	cs := wf["commit_strategy"].(map[string]any)
	assert.Equal(t, "auto", cs["value"], "an absent commit_strategy resolves to the auto default")
	assert.Equal(t, "default", cs["source"])
}

func TestConfigUnrecognizedStrategyWarns(t *testing.T) {
	dir := writeStrategyProject(t, `{"commit_strategy":"squash"}`)

	out, stderr, code := runTPHC(t, dir, "0", "config")
	require.Equal(t, 0, code, "an unrecognized value still exits 0")
	assert.Contains(t, stderr, "squash", "the warning names the unrecognized value")
	var cfg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &cfg))
	assert.Equal(t, "builtin", cfg["commit_strategy_effective"], "an unrecognized value is effectively builtin")
}

func TestProjectCommitStrategySettableAndResolved(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	// §16.2: the project default is now settable via --project and writes
	// workflow.commit_strategy into .tp/config.json.
	out, stderr, code := runTPHC(t, dir, "0", "set", "--workflow", "--project", "commit_strategy=hc")
	require.Equal(t, 0, code, "project commit_strategy is settable: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, "hc", res["updated"].(map[string]any)["commit_strategy"])

	// §16.3: config --resolved attributes commit_strategy to the project layer.
	out, _, code = runTPHC(t, dir, "0", "config", "--resolved")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	wf := res["workflow"].(map[string]any)
	cs := wf["commit_strategy"].(map[string]any)
	assert.Equal(t, "hc", cs["value"], "the project default flows into resolution")
	assert.Equal(t, "project", cs["source"], "the project layer is named")
}

func TestProjectCommitStrategyRejectsInvalidValue(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	_, stderr, code := runTPHC(t, dir, "0", "set", "--workflow", "--project", "commit_strategy=squash")
	require.Equal(t, 1, code, "an invalid value is a validation error")
	assert.Contains(t, stderr, "commit_strategy must be one of builtin, auto, hc")
}

func TestTaskLevelCommitStrategyRefusalNamesProjectSetter(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	// §16.2: the task-file value stays init-authored; task-level set keeps its
	// exit-2 refusal, and the hint now names the project-level setter.
	_, stderr, code := runTPHC(t, dir, "0", "set", "--workflow", "commit_strategy=hc")
	require.Equal(t, 2, code, "task-level commit_strategy keeps its exit-2 refusal")
	assert.Contains(t, stderr, "tp set --workflow --project commit_strategy", "the hint names the project-level setter")
}

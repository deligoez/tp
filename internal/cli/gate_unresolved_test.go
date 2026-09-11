package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// missingGateCmd names a command no host has, so a gate that starts with it
// exits 127 at close on every machine the suite runs on.
const missingGateCmd = "definitely-not-a-cmd-xyz"

// importGateDoc is a wrapped task file whose workflow carries gate.
func importGateDoc(t *testing.T, gate string) string {
	t.Helper()
	wf, err := json.Marshal(map[string]string{"quality_gate": gate})
	require.NoError(t, err)
	return `{"version":1,"spec":"spec.md","workflow":` + string(wf) + `,"tasks":[` + importWorkflowTask + `]}`
}

// importWithGate imports a one-task file carrying gate into a fresh project
// and returns the import's stderr.
func importWithGate(t *testing.T, gate string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(importGateDoc(t, gate)), 0o600))

	stdout, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "an unresolvable gate is a notice, not a refusal: stdout=%s stderr=%s", stdout, stderr)
	return stderr
}

// initWithGate inits a fresh project with gate and returns the directory and
// the init's stderr. setupProjectWithGate discards stderr, which is the
// channel under test here.
func initWithGate(t *testing.T, gate string, files map[string]os.FileMode) (dir, stderr string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Test Spec\n"), 0o600))
	for name, mode := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o600))
		require.NoError(t, os.Chmod(path, mode))
	}
	stdout, stderr, code := runTP(t, dir, "init", "spec.md", "--quality-gate", gate)
	require.Equal(t, 0, code, "an unresolvable gate is a notice, not a refusal: stdout=%s stderr=%s", stdout, stderr)
	return dir, stderr
}

func TestUnresolvedGate_InitWarnsAndNamesTheCommand(t *testing.T) {
	t.Parallel()
	gate := missingGateCmd + " && true"
	dir, stderr := initWithGate(t, gate, nil)

	assert.Contains(t, stderr, "warning", "init must warn when the gate's first command does not resolve")
	assert.Contains(t, stderr, `"`+missingGateCmd+`"`, "the warning must name the command that does not resolve")
	assert.NotContains(t, stderr, "--skip-gate", "a typo in the gate is not a reason to reach for --skip-gate")

	wf := readPersistedWorkflow(t, dir)
	assert.Equal(t, gate, wf["quality_gate"], "a notice, not a refusal: the gate is stored as given")
}

func TestUnresolvedGate_ImportWarnsAndNamesTheCommand(t *testing.T) {
	t.Parallel()
	stderr := importWithGate(t, missingGateCmd+" && true")

	assert.Contains(t, stderr, "warning", "import must warn when the gate's first command does not resolve")
	assert.Contains(t, stderr, `"`+missingGateCmd+`"`, "the warning must name the command that does not resolve")
}

// A gate whose first command resolves — on PATH, a shell builtin, an
// executable path from the project root, or behind an environment
// assignment — must not warn: a notice on every init would train an agent to
// ignore the one that matters.
func TestUnresolvedGate_ResolvableGateDoesNotWarn(t *testing.T) {
	t.Parallel()
	for _, gate := range []string{
		"true",
		"exit 0",
		"FOO=1 true",
		"./ok.sh && true",
		"(cd . && true)",
		"true 2>&1 | " + missingGateCmd, // only the first segment's head is resolved at init
	} {
		t.Run(gate, func(t *testing.T) {
			t.Parallel()
			_, stderr := initWithGate(t, gate, map[string]os.FileMode{"ok.sh": 0o755})
			assert.NotContains(t, stderr, "warning", "gate %q resolves; init must stay silent", gate)
		})
	}
	t.Run("import", func(t *testing.T) {
		t.Parallel()
		stderr := importWithGate(t, "true")
		assert.NotContains(t, stderr, "warning", "gate \"true\" resolves; import must stay silent")
	})
}

// A non-executable file named as the gate's first command is a gate that
// exits 126 at close, so init warns about it just as it does about 127.
func TestUnresolvedGate_InitWarnsOnNonExecutablePath(t *testing.T) {
	t.Parallel()
	_, stderr := initWithGate(t, "./gate.sh", map[string]os.FileMode{"gate.sh": 0o644})
	assert.Contains(t, stderr, "warning")
	assert.Contains(t, stderr, `"./gate.sh"`)
}

// doneGateError closes t1 against gate and returns the parsed error object.
func doneGateError(t *testing.T, gate string, files map[string]os.FileMode) map[string]any {
	t.Helper()
	dir, _ := initWithGate(t, gate, files)
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully")
	require.Equal(t, 4, code, "a failed gate exits 4: %s", stderr)
	var errOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &errOut), "stderr: %s", stderr)
	assert.NotContains(t, stderr, "--skip-gate", "a gate that cannot run is fixed, not skipped: %s", stderr)
	return errOut
}

func TestUnresolvedGate_DoneOnExit127NamesTheCommandWithoutSkipGate(t *testing.T) {
	t.Parallel()
	for _, gate := range []string{
		missingGateCmd + " && true",
		"true && " + missingGateCmd, // the missing command is not the first one
	} {
		t.Run(gate, func(t *testing.T) {
			t.Parallel()
			errOut := doneGateError(t, gate, nil)
			assert.Equal(t, float64(127), errOut["exit_code"])
			assert.Contains(t, errOut["error"], `"`+missingGateCmd+`"`, "the error must name the command that could not run")
			assert.Contains(t, errOut["hint"], "fix the gate command")
			assert.NotContains(t, errOut["hint"], "--skip-gate")
		})
	}
}

func TestUnresolvedGate_DoneOnExit126NamesTheCommandWithoutSkipGate(t *testing.T) {
	t.Parallel()
	errOut := doneGateError(t, "./gate.sh", map[string]os.FileMode{"gate.sh": 0o644})
	assert.Equal(t, float64(126), errOut["exit_code"])
	assert.Contains(t, errOut["error"], `"./gate.sh"`, "the error must name the command that could not run")
	assert.Contains(t, errOut["hint"], "fix the gate command")
}

// The batch path renders its own per-entry failure, and must not steer an
// agent toward skip_gate for a gate that cannot run either.
func TestUnresolvedGate_BatchOnExit127DoesNotSuggestSkipGate(t *testing.T) {
	t.Parallel()
	dir, _ := initWithGate(t, missingGateCmd+" && true", nil)
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	ndjson := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjson, []byte(`{"id":"a","reason":"A complete and verified"}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "done", "--batch", ndjson)
	assert.Equal(t, 4, code, "stderr: %s", stderr)
	assert.NotContains(t, stdout+stderr, "--skip-gate")

	var batchOut struct {
		Failures []struct {
			Error string `json:"error"`
			Hint  string `json:"hint"`
		} `json:"failures"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &batchOut), "stdout: %s", stdout)
	require.Len(t, batchOut.Failures, 1)
	assert.Contains(t, batchOut.Failures[0].Error, `"`+missingGateCmd+`"`)
	assert.Contains(t, batchOut.Failures[0].Hint, "fix the gate command")
}

// A gate that ran and failed keeps today's hint: that failure is the work's,
// and skipping it stays a decision the operator can take.
func TestUnresolvedGate_OrdinaryFailureKeepsSkipGateHint(t *testing.T) {
	t.Parallel()
	dir, _ := initWithGate(t, "exit 1", nil)
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	var errOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &errOut), "stderr: %s", stderr)
	assert.Equal(t, float64(1), errOut["exit_code"])
	assert.Equal(t, "quality gate failed: exit 1", errOut["error"])
	assert.Equal(t, "fix the gate failure and retry, or close with --skip-gate '<why>' (recorded on the task)", errOut["hint"])
	assert.False(t, strings.Contains(errOut["hint"].(string), "fix the gate command"))
}

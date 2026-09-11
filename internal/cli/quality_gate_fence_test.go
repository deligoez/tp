package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The quality gate is the command every close runs, so a unit that can change
// it can replace it with one that always passes: --skip-gate by another route,
// and --skip-gate is fenced. These tests pin the fence at every write path that
// reaches the gate. tp set --workflow refuses the write itself at both layers;
// tp import, tp init --quality-gate and tp config --extract refuse only a
// change to what resolves, because tp run's decompose unit ends in a plain
// tp import that carries the existing block forward, and a value rule there
// would stop every run at its import.

// assertGateFenceRefused checks the refusal's exit code, its wording and its
// escalation route. The negative assertion is v0.37.0 row 14's lesson: the
// command-field refusal says the field "names a command the driver executes",
// which is false for the gate — tp done runs it, not the driver.
func assertGateFenceRefused(t *testing.T, stderr string, code int, label string) {
	t.Helper()
	require.Equal(t, 2, code, "%s must exit 2 under TP_UNATTENDED: %s", label, stderr)
	e := errJSON(t, stderr)
	msg, _ := e["error"].(string)
	hint, _ := e["hint"].(string)
	assert.Contains(t, msg, "the quality gate every close runs", label)
	assert.Contains(t, msg, "user-approved decision", label)
	assert.Contains(t, hint, "tp escalate --decision skip-gate", label)
	assert.NotContains(t, msg+hint, "names a command the driver executes",
		"%s: the driver does not run the gate, so the command-field wording would be false", label)
}

// shellGate is the gate gateShell's task file carries.
const shellGate = "echo ok"

// gateShell is a git-rooted project with one spec whose task file was authored
// by tp init with shellGate, and no tasks yet — the zero-task shell a
// decompose unit imports into.
func gateShell(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))
	_, stderr, code := runTPFence(t, dir, false, "init", "spec.md", "--quality-gate", shellGate)
	require.Equal(t, 0, code, "init: %s", stderr)
	return dir
}

// gateResolved reads the quality gate a base resolves, through config
// --resolved, so the assertion is on what a close would run rather than on a
// file's bytes.
func gateResolved(t *testing.T, dir, taskFile string) string {
	t.Helper()
	args := []string{"config", "--resolved"}
	if taskFile != "" {
		args = append(args, "--file", taskFile)
	}
	out, stderr, code := runTPFence(t, dir, false, args...)
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	wf, ok := res["workflow"].(map[string]any)
	require.True(t, ok, "config --resolved carries a workflow object")
	entry, ok := wf["quality_gate"].(map[string]any)
	require.True(t, ok, "config --resolved reports quality_gate")
	value, _ := entry["value"].(string)
	return value
}

func TestQualityGateFence_SetRefusedAtBothLayers(t *testing.T) {
	t.Parallel()
	dir := gateShell(t)
	before := extractTreeState(t, dir)

	_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "--project", "quality_gate=true")
	assertGateFenceRefused(t, stderr, code, "set --workflow --project")
	assert.Equal(t, before, extractTreeState(t, dir), "the refused project write reached no file")

	_, stderr, code = runTPFence(t, dir, true, "set", "--workflow", "quality_gate=true")
	assertGateFenceRefused(t, stderr, code, "set --workflow")
	assert.Equal(t, "echo ok", gateResolved(t, dir, "spec.tasks.json"), "the gate a close runs is unchanged")

	_, stderr, code = runTPFence(t, dir, false, "set", "--workflow", "--project", "quality_gate=true")
	require.Equal(t, 0, code, "attended, the project gate is the operator's to set: %s", stderr)
}

func TestQualityGateFence_ImportRefusesOnlyAChange(t *testing.T) {
	t.Parallel()
	doc := func(workflow string) string {
		d := `{"version":1,"spec":"spec.md","tasks":[` +
			`{"id":"n1","title":"New","estimate_minutes":5,"acceptance":"New task complete.","source_sections":["## 1. Setup"],"depends_on":[]}]`
		if workflow != "" {
			d += `,"workflow":` + workflow
		}
		return d + "}"
	}
	importDoc := func(t *testing.T, dir, body string) (string, int) {
		t.Helper()
		path := filepath.Join(dir, "import.json")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		_, stderr, code := runTPFence(t, dir, true, "import", path)
		return stderr, code
	}

	t.Run("a document that changes the gate is refused", func(t *testing.T) {
		dir := gateShell(t)
		before := extractTreeState(t, dir)
		stderr, code := importDoc(t, dir, doc(`{"quality_gate":"true"}`))
		assertGateFenceRefused(t, stderr, code, "import")
		assert.Equal(t, before, extractTreeState(t, dir), "the refused import reached no file")
		assert.Equal(t, "echo ok", gateResolved(t, dir, "spec.tasks.json"))
	})

	t.Run("a document that carries the block forward passes", func(t *testing.T) {
		dir := gateShell(t)
		stderr, code := importDoc(t, dir, doc(""))
		require.Equal(t, 0, code, "omitting the workflow key carries the gate forward: %s", stderr)
		assert.Equal(t, "echo ok", gateResolved(t, dir, "spec.tasks.json"))
		assert.Equal(t, "open", showTask(t, dir, "n1")["status"], "and the import completed")
	})

	t.Run("a document naming the resolved gate passes", func(t *testing.T) {
		dir := gateShell(t)
		stderr, code := importDoc(t, dir, doc(`{"quality_gate":"echo ok"}`))
		require.Equal(t, 0, code, "the same gate is no change: %s", stderr)
		assert.Equal(t, "echo ok", gateResolved(t, dir, "spec.tasks.json"))
	})
}

func TestQualityGateFence_InitRefusesOnlyAChange(t *testing.T) {
	t.Parallel()
	dir := extractShell(t, `{"quality_gate":"echo ok"}`, extractBase{"a", ""}, extractBase{"b", ""})

	_, stderr, code := runTPFence(t, dir, true, "init", "a.md", "--quality-gate", "true")
	assertGateFenceRefused(t, stderr, code, "init --quality-gate")
	_, statErr := os.Stat(filepath.Join(dir, "a.tasks.json"))
	assert.True(t, os.IsNotExist(statErr), "the refused init wrote no task file")

	_, stderr, code = runTPFence(t, dir, true, "init", "a.md", "--quality-gate", "echo ok")
	require.Equal(t, 0, code, "naming the gate the base already resolves is no change: %s", stderr)

	_, stderr, code = runTPFence(t, dir, true, "init", "b.md")
	require.Equal(t, 0, code, "an init that names no gate inherits it: %s", stderr)
	assert.Equal(t, "echo ok", gateResolved(t, dir, "b.tasks.json"))
}

func TestQualityGateFence_ExtractRefusesAHoistThatMovesTheGate(t *testing.T) {
	t.Parallel()
	t.Run("a hoist that moves the gate of a base with no task file", func(t *testing.T) {
		dir := extractShell(t, "",
			extractBase{"a", `{"quality_gate":"true"}`},
			extractBase{"b", `{"quality_gate":"true"}`},
			extractBase{"c", ""})
		before := extractTreeState(t, dir)

		_, stderr, code := runTPFence(t, dir, true, "config", "--extract")
		assertGateFenceRefused(t, stderr, code, "config --extract")
		assert.Equal(t, before, extractTreeState(t, dir), "the refused hoist reached no file")
	})

	t.Run("a hoist beneath a project layer that already resolves the gate passes", func(t *testing.T) {
		dir := extractShell(t, `{"quality_gate":"true"}`,
			extractBase{"a", `{"quality_gate":"true"}`},
			extractBase{"b", `{"quality_gate":"true"}`},
			extractBase{"c", ""})
		out, stderr, code := runTPFence(t, dir, true, "config", "--extract", "--force")
		require.Equal(t, 0, code, "the hoist changes no resolved gate: %s", stderr)
		var res map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &res))
		assert.Contains(t, res["hoisted"], "quality_gate", "the gate was moved, so the fence had a change to grade")
	})

	t.Run("attended, the same hoist completes", func(t *testing.T) {
		dir := extractShell(t, "",
			extractBase{"a", `{"quality_gate":"true"}`},
			extractBase{"b", `{"quality_gate":"true"}`},
			extractBase{"c", ""})
		_, stderr, code := runTPFence(t, dir, false, "config", "--extract")
		require.Equal(t, 0, code, "the operator may hoist the gate: %s", stderr)
	})
}

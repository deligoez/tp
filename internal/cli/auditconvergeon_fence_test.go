package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTPFence runs tp with TP_UNATTENDED decided by this call rather than
// inherited from whatever started the suite.
//
// v0.37.0 §7's closing note is why it exists: runTPEnv builds every child from
// os.Environ(), and under tp run the quality gate itself runs in a child
// carrying TP_UNATTENDED=1 — so a fence test that relied on the ambient value
// would assert the opposite of what it names on half the machines that run it.
// Row 13c's "with TP_UNATTENDED unset" cannot be reached by omitting the
// variable at all. The variable is filtered out of the inherited environment
// first and re-added only when unattended is true, so both arms are pinned
// rather than one.
func runTPFence(t *testing.T, dir string, unattended bool, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+3)
	for _, kv := range inherited {
		if strings.HasPrefix(kv, "TP_UNATTENDED=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "NO_COLOR=1", "TP_HC=0")
	if unattended {
		env = append(env, "TP_UNATTENDED=1")
	}

	cmd := exec.Command(binaryPath, append([]string{"--json"}, args...)...)
	cmd.Dir = dir
	cmd.Env = env

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

// fenceShell builds a zero-task cycle for §3's fence rows: a spec s.md with one
// section, a zero-task s.tasks.json carrying taskWorkflow, and — when
// projectWorkflow is non-empty — a .tp/config.json carrying it.
//
// The shell is deliberately zero-task. §7 row 13b records that over a
// tasks-bearing target tp import exits 3 at the exists-guard before the fence is
// reached, so an import row built over a populated target passes green with no
// fence built at all.
func fenceShell(t *testing.T, taskWorkflow, projectWorkflow string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.md"),
		[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"version":1,"spec":"s.md","tasks":[],"workflow":`+taskWorkflow+`}`), 0o600))
	if projectWorkflow != "" {
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
			[]byte(`{"workflow":`+projectWorkflow+`}`), 0o600))
	}
	return dir
}

// fenceImportDoc writes an importable document carrying one task that covers the
// shell's only section, with the given top-level workflow text. A workflow of ""
// omits the key entirely, which is the shape §3's first authoring remedy names
// and the shape that makes tp import preserve the existing block.
func fenceImportDoc(t *testing.T, dir, workflow string) string {
	t.Helper()
	const name = "doc.json"
	doc := `{"version":1,"spec":"s.md","tasks":[{"id":"t1","title":"T",` +
		`"estimate_minutes":5,"acceptance":"Done.","source_sections":["## 1. Setup"]}]`
	if workflow != "" {
		doc += `,"workflow":` + workflow
	}
	doc += `}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(doc), 0o600))
	return name
}

// fenceResolved returns the audit_converge_on value tp resolves in dir, read
// through tp config --resolved so the assertion cannot disagree with what an
// operator sees when they go and look. It runs attended, because reading is not
// a write and §3 fences only writes.
func fenceResolved(t *testing.T, dir string) string {
	t.Helper()
	out, stderr, code := runTPFence(t, dir, false, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	wf, ok := res["workflow"].(map[string]any)
	require.True(t, ok, "config --resolved carries a workflow object")
	entry, ok := wf["audit_converge_on"].(map[string]any)
	require.True(t, ok, "config --resolved reports audit_converge_on")
	value, ok := entry["value"].(string)
	require.True(t, ok, "the entry carries a string value")
	return value
}

// assertFenceRefused asserts one sink's refusal under §3's change rule. The
// exit code is asserted twice — as the process status and as the envelope's own
// field — for the reason the illegal-literal helper gives: a sink that exits 2
// while reporting another code in its JSON tells an agent the wrong story.
//
// The negative assertion is row 14's mutant in miniature: reusing
// refuseUnattendedCommandField would interpolate this field's name and so
// satisfy any assertion phrased only as "the message names the field".
func assertFenceRefused(t *testing.T, stderr string, code int, sink string) {
	t.Helper()
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code, "%s: a relax under TP_UNATTENDED exits 2", sink)
	assert.Equal(t, float64(2), e["code"], "%s: and the envelope reports the same code", sink)
	msg, ok := e["error"].(string)
	require.True(t, ok, "%s: the envelope carries a message", sink)
	assert.Contains(t, msg, "audit_converge_on", "%s: the refusal names the field", sink)
	assert.NotContains(t, msg, "names a command the driver executes",
		"%s: and does not reuse the command-field refusal's wording", sink)
}

// TestAuditConvergeOnFence_RelaxRefusedAtEveryWriteSink covers v0.37.0 §7 row
// 12: under TP_UNATTENDED=1 a write that changes the resolved value to blocking
// exits 2 at tp set --workflow, at its --project form, and at tp import — three
// assertions, one per path, because the mutant row 12 names is fencing one file
// and leaving the others open. Each subtest also asserts the tree was left as it
// was found, which is what separates a sink that refuses from one that refuses
// after writing.
func TestAuditConvergeOnFence_RelaxRefusedAtEveryWriteSink(t *testing.T) {
	t.Run("set --workflow", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")
		taskFile := filepath.Join(dir, "s.tasks.json")
		before, err := os.ReadFile(taskFile)
		require.NoError(t, err)

		_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "audit_converge_on=blocking")
		assertFenceRefused(t, stderr, code, "set --workflow")

		after, err := os.ReadFile(taskFile)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "the refused write reached no file")
		assert.Equal(t, "all", fenceResolved(t, dir), "and nothing resolves differently")
	})

	t.Run("set --workflow --project", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")

		_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "--project", "audit_converge_on=blocking")
		assertFenceRefused(t, stderr, code, "set --workflow --project")

		_, statErr := os.Stat(filepath.Join(dir, ".tp", "config.json"))
		assert.True(t, os.IsNotExist(statErr), "the refused write created no project config")
		assert.Equal(t, "all", fenceResolved(t, dir), "and nothing resolves differently")
	})

	t.Run("import", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")
		doc := fenceImportDoc(t, dir, `{"audit_converge_on":"blocking"}`)

		_, stderr, code := runTPFence(t, dir, true, "import", doc)
		assertFenceRefused(t, stderr, code, "import")

		assert.Equal(t, "all", fenceResolved(t, dir), "the refused import reached no file")
	})

	// §3: "writing all first and blocking second is refused on the second
	// write, so there is no walk-around". A transition-shaped fence keyed on
	// all → blocking would pass the first write and refuse the second too, so
	// this is not the row that discriminates — row 13b is. It is here because
	// §3 states it, and because a change rule that consulted the *stored*
	// value rather than the resolved one would leave the second write open
	// after the first had written nothing.
	t.Run("no walk-around through an explicit all", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")

		_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "audit_converge_on=all")
		require.Equal(t, 0, code, "writing the default is not a relax: %s", stderr)

		_, stderr, code = runTPFence(t, dir, true, "set", "--workflow", "audit_converge_on=blocking")
		assertFenceRefused(t, stderr, code, "second write")
		assert.Equal(t, "all", fenceResolved(t, dir), "the second write changed nothing")
	})
}


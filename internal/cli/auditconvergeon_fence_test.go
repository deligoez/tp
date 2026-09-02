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
	return fenceResolvedBase(t, dir, "")
}

// fenceResolvedBase is the same reading addressed at one base rather than at
// the active pointer: taskFile names the base's task file, which need not
// exist. A --file naming a task file that is absent discovers nothing, so the
// override layer is empty and the project layer alone decides — which is
// exactly how §3's third spec, the base with no task file, resolves.
func fenceResolvedBase(t *testing.T, dir, taskFile string) string {
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

	// §7 row 14: the decision the hint names is the field's own, not the
	// fallback. A run stopped under `other` is recoverable only from the free
	// text of --evidence, so the hint sending a unit there is the difference
	// between a nameable stop and an unnameable one.
	hint, ok := e["hint"].(string)
	require.True(t, ok, "%s: the envelope carries a hint", sink)
	assert.Contains(t, hint, "--decision audit-converge-on",
		"%s: the hint names the field's own decision", sink)
	assert.NotContains(t, hint, "--decision other",
		"%s: and not the fallback", sink)
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

// TestAuditConvergeOnFence_WritesThatChangeNothingPass covers v0.37.0 §7 row 13
// — the three writes that must pass under the same environment. Row 13's mutant
// is a field-scoped or value-scoped fence: either refuses the third case here
// and deadlocks Workflow A step 6 for the opt-in users this release exists for,
// which is the whole reason §3 is a change rule.
func TestAuditConvergeOnFence_WritesThatChangeNothingPass(t *testing.T) {
	t.Run("a write of all at both set sinks", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")

		_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "audit_converge_on=all")
		require.Equal(t, 0, code, "task sink: %s", stderr)

		_, stderr, code = runTPFence(t, dir, true, "set", "--workflow", "--project", "audit_converge_on=all")
		require.Equal(t, 0, code, "project sink: %s", stderr)

		assert.Equal(t, "all", fenceResolved(t, dir), "all is the default and tightens nothing")
	})

	// §3: "a write of all never trips it, since all is the default and the only
	// value that tightens the gate." The direction is the assertion. A rule
	// phrased as "the resolved value changed" rather than "changed to
	// blocking" refuses this write and leaves a unit unable to tighten its own
	// gate — a fence pointed the wrong way, which the three mutants §7 names
	// for these rows all leave standing.
	t.Run("a write of all over an already-resolved blocking", func(t *testing.T) {
		dir := fenceShell(t, `{"audit_converge_on":"blocking"}`, "")
		require.Equal(t, "blocking", fenceResolved(t, dir), "the shell starts out opted in")

		_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "audit_converge_on=all")
		require.Equal(t, 0, code, "tightening the gate is never a user-approved decision: %s", stderr)
		assert.Equal(t, "all", fenceResolved(t, dir), "and the tightening landed")
	})

	// §3: "'Resolved' means resolved, not stored at the written layer." The
	// task override wins, so this write changes nothing an audit will read.
	// A fence comparing the value against the layer it was written to refuses
	// this and is the reason the clause is in the spec at all.
	t.Run("a project write of blocking beneath a task override of all", func(t *testing.T) {
		dir := fenceShell(t, `{"audit_converge_on":"all"}`, "")

		_, stderr, code := runTPFence(t, dir, true, "set", "--workflow", "--project", "audit_converge_on=blocking")
		require.Equal(t, 0, code, "the write is covered and changes no resolution: %s", stderr)

		data, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
		require.NoError(t, err)
		assert.Contains(t, string(data), `"audit_converge_on": "blocking"`,
			"the value reached the project layer it was addressed to")
		assert.Equal(t, "all", fenceResolved(t, dir),
			"and the task override still decides what an audit reads")
	})

	// §3: "an import carrying an already-resolved blocking forward changes
	// nothing and passes". The document omits the top-level workflow key, so
	// tp import's preservation step carries the existing block over — which is
	// exactly the import a field-scoped or value-scoped fence would refuse
	// every time, for ever, once a project has opted in.
	t.Run("an import carrying an already-resolved blocking forward", func(t *testing.T) {
		dir := fenceShell(t, `{"audit_converge_on":"blocking"}`, "")
		require.Equal(t, "blocking", fenceResolved(t, dir), "the shell starts out opted in")
		doc := fenceImportDoc(t, dir, "")

		_, stderr, code := runTPFence(t, dir, true, "import", doc)
		require.Equal(t, 0, code, "the import changes no resolved value: %s", stderr)

		assert.Equal(t, "blocking", fenceResolved(t, dir),
			"and the block it carried forward is still what resolves")
	})
}

// TestAuditConvergeOnFence_SetProjectEvaluatesEveryBase is the --project sink's
// half of §3's change rule, and it was run against the single-base fence first
// and observed passing there: two task files, one carrying an override of all
// and one carrying an empty workflow block, with the active pointer naming the
// first, over which `TP_UNATTENDED=1 tp set --workflow --project
// audit_converge_on=blocking` exited 0 and moved the second base from
// default/all to project/blocking with no refusal and no escalation record.
//
// The active pointer is load-bearing rather than scenery. Without it two task
// files make discovery ambiguous, the discovered override is empty, and the
// unwidened fence refuses on that empty override alone — so the same tree would
// pass green against the code this test exists to fail.
//
// The population is every base the fence can observe: the discovered override
// plus every scanned task file's own. It does NOT carry --extract's
// unconditional empty override, and §7 row 13 is why — measured, appending it
// refuses the sibling subtest above, where the one base in the tree carries a
// task override of all and the write is covered. A base with no task file at
// all is therefore still outside this sink's population; tp scans task files and
// has no enumeration of specs, so that base has no shape the fence can see.
func TestAuditConvergeOnFence_SetProjectEvaluatesEveryBase(t *testing.T) {
	dir := extractShell(t, "",
		extractBase{"a", `{"audit_converge_on":"all"}`},
		extractBase{"b", "{}"})
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "local.json"),
		[]byte(`{"active":"a.tasks.json"}`), 0o600))
	require.Equal(t, "all", fenceResolvedBase(t, dir, "a.tasks.json"),
		"the covered base resolves its own override")
	require.Equal(t, "all", fenceResolvedBase(t, dir, "b.tasks.json"),
		"and the base whose block names no override resolves the default")

	_, stderr, code := runTPFence(t, dir, true,
		"set", "--workflow", "--project", "audit_converge_on=blocking")
	assertFenceRefused(t, stderr, code, "set --workflow --project over an uncovered base")

	_, statErr := os.Stat(filepath.Join(dir, ".tp", "config.json"))
	assert.True(t, os.IsNotExist(statErr), "the refused write created no project config")
	assert.Equal(t, "all", fenceResolvedBase(t, dir, "b.tasks.json"),
		"and the uncovered base still resolves the default")
}

// TestAuditConvergeOnFence_ImportUncoversProjectBlocking covers v0.37.0 §7 row
// 13b, the case that discriminates the change rule from every narrower one. The
// imported document names neither literal: it carries a workflow key holding
// only review_max_rounds, which makes tp import replace the task-level block
// wholesale, drop the task-level all, and let the project-level blocking become
// what resolves. A fence keyed on the field, on the value written, or on an
// all → blocking transition passes this document, so this is the row that fails
// under all three mutants at once.
//
// The zero-task shell is load-bearing and is asserted rather than assumed by the
// sibling below: over a tasks-bearing target the exists-guard exits 3 first.
func TestAuditConvergeOnFence_ImportUncoversProjectBlocking(t *testing.T) {
	dir := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
	require.Equal(t, "all", fenceResolved(t, dir),
		"the task override covers the project block before the import")
	doc := fenceImportDoc(t, dir, `{"review_max_rounds":5}`)

	_, stderr, code := runTPFence(t, dir, true, "import", doc)
	assertFenceRefused(t, stderr, code, "import uncovering the project layer")

	assert.Equal(t, "all", fenceResolved(t, dir), "and the block stayed covered")
}

// TestAuditConvergeOnFence_ImportExistsGuardPrecedesTheFence asserts row 13b's
// precondition rather than assuming it, which is the sibling the test above
// names.
//
// §7 row 13b records that over a tasks-bearing target tp import exits 3 at the
// exists-guard before the fence is reached. That is not a footnote about
// fixture hygiene: an import row built over a populated target is refused
// either way, so an assertion phrased as "the import is refused" passes green
// with no fence built at all. 3 and 2 are different answers, and this test
// makes the difference the assertion — the exit code, the guard that produced
// it, and the absence of the fence's own wording from a message that would
// otherwise be indistinguishable to an agent reading only "refused".
//
// The closing half runs the same document over the zero-task shell, so the two
// answers are a discrimination made in one place rather than a claim about
// what some other test would have seen.
func TestAuditConvergeOnFence_ImportExistsGuardPrecedesTheFence(t *testing.T) {
	dir := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
	// Everything but the target's emptiness is row 13b's fixture.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"version":1,"spec":"s.md","tasks":[{"id":"t0","title":"T0","status":"open",`+
			`"estimate_minutes":5,"acceptance":"Already here.","source_sections":["## 1. Setup"]}],`+
			`"workflow":{"audit_converge_on":"all"}}`), 0o600))
	doc := fenceImportDoc(t, dir, `{"review_max_rounds":5}`)

	_, stderr, code := runTPFence(t, dir, true, "import", doc)

	e := errJSON(t, stderr)
	require.Equal(t, 3, code, "the exists-guard answers first, and 3 is not 2: %s", stderr)
	assert.Equal(t, float64(3), e["code"], "and the envelope reports the same code")
	msg, ok := e["error"].(string)
	require.True(t, ok, "the envelope carries a message")
	assert.Contains(t, msg, "task file already exists",
		"it is the exists-guard that answered")
	assert.NotContains(t, msg, "audit_converge_on",
		"and the fence was never reached, so it cannot be what refused")
	assert.Equal(t, "all", fenceResolved(t, dir), "the refused import reached no file")

	// The same document, the same environment, the same two layers — only the
	// target's emptiness differs, and that is what moves the answer from the
	// exists-guard to the fence.
	shell := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
	shellDoc := fenceImportDoc(t, shell, `{"review_max_rounds":5}`)
	_, stderr, code = runTPFence(t, shell, true, "import", shellDoc)
	assertFenceRefused(t, stderr, code, "the same document over a zero-task shell")
}

// extractBase names one base in an --extract fixture: a spec, plus a task file
// carrying workflow when workflow is non-empty. A base whose workflow text is
// empty gets a spec and no task file at all — §3's third spec, and the base the
// scanned-file population cannot see.
type extractBase struct {
	name     string
	workflow string
}

// extractShell builds an --extract fixture: one spec per base, a task file for
// each base that names a workflow block, and — when projectWorkflow is
// non-empty — a .tp/config.json carrying it.
func extractShell(t *testing.T, projectWorkflow string, bases ...extractBase) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	for _, b := range bases {
		require.NoError(t, os.WriteFile(filepath.Join(dir, b.name+".md"),
			[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))
		if b.workflow == "" {
			continue
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, b.name+".tasks.json"),
			[]byte(`{"version":1,"spec":"`+b.name+`.md","tasks":[],"workflow":`+b.workflow+`}`), 0o600))
	}
	if projectWorkflow != "" {
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
			[]byte(`{"workflow":`+projectWorkflow+`}`), 0o600))
	}
	return dir
}

// extractTreeState returns the bytes of every file --extract writes: each task
// file it would thin, and the project config it would merge into. A refusal
// asserted only as an exit code cannot tell a sink that refuses from one that
// refuses after writing, and the second is the mutant this helper exists to
// kill.
func extractTreeState(t *testing.T, dir string) map[string]string {
	t.Helper()
	state := make(map[string]string)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".tasks.json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, readErr)
		state[e.Name()] = string(data)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
	if readErr == nil {
		state[".tp/config.json"] = string(data)
	} else {
		require.True(t, os.IsNotExist(readErr), "the project config is either readable or absent")
	}
	return state
}

// extractBlockingBlock is the task-level block §3 measured: the opt-in this
// release exists for, plus one unfenced field so the hoist has work to do
// either way.
const extractBlockingBlock = `{"audit_converge_on":"blocking","review_max_rounds":7}`

// TestAuditConvergeOnFence_ExtractEvaluatesEveryBase is §3's measured
// counterexample, and it was run against the unwidened fence first and observed
// passing there before a line of the fix was written: two task files carrying
// blocking and one spec with no task file, over which `TP_UNATTENDED=1 tp config
// --extract` exited 0, hoisted audit_converge_on, and moved the third spec from
// default/all to project/blocking.
//
// The population §3 requires is every base the repository resolves. It is
// implemented as the scanned overrides plus ONE empty override rather than one
// per spec, because engine.ResolveWorkflowLayers is a pure function of the two
// layers it is handed — no base contributes anything else — so every base
// without a task-level override resolves identically and enumerating the
// repository's specs would run the same comparison once per spec.
//
// The empty override is appended unconditionally, and the third subtest is
// where that decision is paid for and asserted rather than left implicit.
func TestAuditConvergeOnFence_ExtractEvaluatesEveryBase(t *testing.T) {
	t.Run("a base with no task file", func(t *testing.T) {
		dir := extractShell(t, "",
			extractBase{"a", extractBlockingBlock},
			extractBase{"b", extractBlockingBlock},
			extractBase{"c", ""})
		require.Equal(t, "all", fenceResolvedBase(t, dir, "c.tasks.json"),
			"the third spec resolves the default before the hoist")
		before := extractTreeState(t, dir)

		_, stderr, code := runTPFence(t, dir, true, "config", "--extract")
		assertFenceRefused(t, stderr, code, "config --extract")

		assert.Equal(t, "all", fenceResolvedBase(t, dir, "c.tasks.json"),
			"and the base with no task file still resolves it")
		assert.Equal(t, before, extractTreeState(t, dir),
			"the refused hoist reached neither the project config nor a task file")
	})

	// §3: "--force does not exempt it; it is not tp import --force, which is a
	// named unattended decision, and it must not become one silently." The
	// first half of this subtest is what stops it passing green on the wrong
	// guard: without --force the same tree exits 4 at the exists-guard, which
	// an assertion phrased only as "refused" cannot tell from the fence's 2.
	t.Run("--force is not an exemption", func(t *testing.T) {
		dir := extractShell(t, `{"review_clean_rounds":3}`,
			extractBase{"a", extractBlockingBlock},
			extractBase{"b", extractBlockingBlock},
			extractBase{"c", ""})
		before := extractTreeState(t, dir)

		_, stderr, code := runTPFence(t, dir, true, "config", "--extract")
		e := errJSON(t, stderr)
		require.Equal(t, 4, code, "without --force the exists-guard answers first: %s", stderr)
		msg, ok := e["error"].(string)
		require.True(t, ok, "the envelope carries a message")
		assert.Contains(t, msg, "already exists", "and it is the exists-guard that answered")
		assert.NotContains(t, msg, "audit_converge_on",
			"so the fence was never reached and cannot be what refused")

		_, stderr, code = runTPFence(t, dir, true, "config", "--extract", "--force")
		assertFenceRefused(t, stderr, code, "config --extract --force")

		assert.Equal(t, "all", fenceResolvedBase(t, dir, "c.tasks.json"),
			"the base with no task file still resolves the default")
		assert.Equal(t, before, extractTreeState(t, dir),
			"and --force reached no file either")
	})

	// The cost of appending the empty override unconditionally, stated as an
	// assertion rather than left to be discovered. Here every base already has
	// a task file and every one of them carries blocking, so the hoist changes
	// nothing that resolves today — and it is still refused.
	//
	// That is deliberate. Whether a spec currently lacks a task file is a
	// property of the tree's shape at this instant, not of the write, and §3's
	// prospective case is exactly what that shape hides: after the hoist the
	// next tp init or tp import writes a task file whose empty workflow block
	// resolves blocking from the project layer, and the later import compares
	// equal and passes. A fence conditioned on the shape would wave this
	// through; the routes out are an attended operator or tp escalate
	// --decision audit-converge-on.
	t.Run("no base without a task file, and still refused", func(t *testing.T) {
		dir := extractShell(t, "",
			extractBase{"a", extractBlockingBlock},
			extractBase{"b", extractBlockingBlock})
		before := extractTreeState(t, dir)

		_, stderr, code := runTPFence(t, dir, true, "config", "--extract")
		assertFenceRefused(t, stderr, code, "config --extract over a fully covered tree")
		assert.Equal(t, before, extractTreeState(t, dir), "and it reached no file")
	})
}

// TestAuditConvergeOnFence_ExtractPassesWhatPreservesResolution pins the other
// direction at the fourth write path, because a fence that refused every
// --extract would satisfy every assertion above while shipping a command nobody
// can run. Both halves complete the hoist rather than merely exiting 0, and the
// second runs under --force, so "--force is not an exemption" above cannot be
// satisfied by a --force that is simply always refused.
func TestAuditConvergeOnFence_ExtractPassesWhatPreservesResolution(t *testing.T) {
	t.Run("a hoist that does not move the fenced field", func(t *testing.T) {
		dir := extractShell(t, "",
			extractBase{"a", `{"review_max_rounds":7}`},
			extractBase{"b", `{"review_max_rounds":7}`},
			extractBase{"c", ""})

		out, stderr, code := runTPFence(t, dir, true, "config", "--extract")
		require.Equal(t, 0, code, "the hoist moves no fenced field: %s", stderr)
		var res map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &res))
		assert.Contains(t, res["hoisted"], "review_max_rounds",
			"the common field was hoisted, so the fence ran on a command that had work to do")
		assert.NotContains(t, res["hoisted"], "audit_converge_on",
			"and the fenced field was not among the ones it moved")
		assert.Equal(t, "all", fenceResolvedBase(t, dir, "c.tasks.json"),
			"so no base resolves the field differently")
	})

	// --dry-run writes nothing, so it changes no resolved value and §3's rule
	// has nothing to refuse — which is why the fence sits past the --dry-run
	// return. That placement is load-bearing and was previously unasserted:
	// a copy of this tree with the fence moved one block earlier turned the
	// read-only preview into an exit 2 while every other test in this file
	// stayed green, and under tp run that stops a run over a *read*. The
	// closing half is what makes this subtest about --dry-run rather than
	// about a tree nothing refuses: the same tree, the same environment, no
	// --dry-run, is refused.
	t.Run("--dry-run previews a hoist the fence would refuse", func(t *testing.T) {
		dir := extractShell(t, "",
			extractBase{"a", extractBlockingBlock},
			extractBase{"b", extractBlockingBlock},
			extractBase{"c", ""})
		before := extractTreeState(t, dir)

		out, stderr, code := runTPFence(t, dir, true, "config", "--extract", "--dry-run")
		require.Equal(t, 0, code, "a preview writes nothing, so it relaxes nothing: %s", stderr)
		var res map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &res))
		assert.Equal(t, true, res["dry_run"], "and it reports itself as the preview")
		assert.Contains(t, res["hoisted"], "audit_converge_on",
			"the preview still names the fenced field it would move")
		assert.Equal(t, before, extractTreeState(t, dir), "and it reached no file")
		assert.Equal(t, "all", fenceResolvedBase(t, dir, "c.tasks.json"),
			"so no base resolves differently after a preview")

		_, stderr, code = runTPFence(t, dir, true, "config", "--extract")
		assertFenceRefused(t, stderr, code, "the same tree without --dry-run")
	})

	// The one hoist of blocking that changes nothing for any base: the project
	// layer already resolves it, so the base with no task file reads blocking
	// before the hoist and blocking after it.
	t.Run("a hoist of blocking beneath a project layer that already resolves it", func(t *testing.T) {
		dir := extractShell(t, `{"audit_converge_on":"blocking"}`,
			extractBase{"a", extractBlockingBlock},
			extractBase{"b", extractBlockingBlock},
			extractBase{"c", ""})
		require.Equal(t, "blocking", fenceResolvedBase(t, dir, "c.tasks.json"),
			"every base is already opted in before the hoist")

		out, stderr, code := runTPFence(t, dir, true, "config", "--extract", "--force")
		require.Equal(t, 0, code, "the hoist changes no resolved value: %s", stderr)
		var res map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &res))
		assert.Contains(t, res["hoisted"], "audit_converge_on",
			"the fenced field is one of the ones it moved, so the fence had a change to grade")

		config, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
		require.NoError(t, err)
		assert.Contains(t, string(config), `"audit_converge_on": "blocking"`,
			"and the command completed rather than aborting part-written")
		assert.NotContains(t, extractTreeState(t, dir)["a.tasks.json"], "audit_converge_on",
			"the thinning ran too")
		assert.Equal(t, "blocking", fenceResolvedBase(t, dir, "c.tasks.json"),
			"and every base still resolves what it resolved before")
	})
}

// TestAuditConvergeOnFence_ImportComparesItsOwnTarget pins which file the
// import fence resolves. §7 names no mutant for this, and it is here because
// building one showed every row above surviving it: tp import writes the file
// its own document's spec names, while every other sink in §3 resolves the
// active pointer, and in a one-cycle fixture those are the same path — so a
// fence that read the active pointer passed rows 12, 13, 13b and 13c unchanged.
//
// Two cycles separate them. The active pointer a.tasks.json is already opted in;
// the import's own target b.tasks.json is not, and the document relaxes it. A
// fence comparing the active pointer sees blocking before and blocking after and
// waves the relax through.
func TestAuditConvergeOnFence_ImportComparesItsOwnTarget(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	for _, base := range []string{"a", "b"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"),
			[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.tasks.json"),
		[]byte(`{"version":1,"spec":"a.md","tasks":[],"workflow":{"audit_converge_on":"blocking"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.tasks.json"),
		[]byte(`{"version":1,"spec":"b.md","tasks":[],"workflow":{}}`), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "local.json"),
		[]byte(`{"active":"a.tasks.json"}`), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "doc.json"),
		[]byte(`{"version":1,"spec":"b.md","tasks":[{"id":"t1","title":"T",`+
			`"estimate_minutes":5,"acceptance":"Done.","source_sections":["## 1. Setup"]}],`+
			`"workflow":{"audit_converge_on":"blocking"}}`), 0o600))

	_, stderr, code := runTPFence(t, dir, true, "import", "doc.json")
	assertFenceRefused(t, stderr, code, "import against its own target")

	data, err := os.ReadFile(filepath.Join(dir, "b.tasks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "blocking", "the refused import reached no file")
}

// TestAuditConvergeOnFence_UnsetEnvironmentRefusesNothing covers v0.37.0 §7 row
// 13c: with TP_UNATTENDED unset, every write rows 12 and 13b refuse succeeds.
// The mutant is scoping the fence to the field rather than to the environment,
// which ships a knob nobody can turn — §3's whole point being that this is a
// human decision, not a forbidden one.
//
// The variable is removed from the child's environment rather than merely not
// added, so this test states its own environment on a machine whose suite runs
// under TP_UNATTENDED=1.
func TestAuditConvergeOnFence_UnsetEnvironmentRefusesNothing(t *testing.T) {
	t.Run("set --workflow", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")
		_, stderr, code := runTPFence(t, dir, false, "set", "--workflow", "audit_converge_on=blocking")
		require.Equal(t, 0, code, "an attended operator writes blocking: %s", stderr)
		assert.Equal(t, "blocking", fenceResolved(t, dir), "and it resolves")
	})

	t.Run("set --workflow --project", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")
		_, stderr, code := runTPFence(t, dir, false, "set", "--workflow", "--project", "audit_converge_on=blocking")
		require.Equal(t, 0, code, "an attended operator writes blocking: %s", stderr)
		assert.Equal(t, "blocking", fenceResolved(t, dir), "and it resolves")
	})

	t.Run("import", func(t *testing.T) {
		dir := fenceShell(t, "{}", "")
		doc := fenceImportDoc(t, dir, `{"audit_converge_on":"blocking"}`)
		_, stderr, code := runTPFence(t, dir, false, "import", doc)
		require.Equal(t, 0, code, "an attended operator imports blocking: %s", stderr)
		assert.Equal(t, "blocking", fenceResolved(t, dir), "and it resolves")
	})

	t.Run("import uncovering the project layer", func(t *testing.T) {
		dir := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
		doc := fenceImportDoc(t, dir, `{"review_max_rounds":5}`)
		_, stderr, code := runTPFence(t, dir, false, "import", doc)
		require.Equal(t, 0, code, "row 13b's document is refused only under the variable: %s", stderr)
		assert.Equal(t, "blocking", fenceResolved(t, dir), "and the project block is uncovered")
	})
}

// TestAuditConvergeOnFence_ImportRemediesPass covers v0.37.0 §7 row 13d: over
// row 13b's own shell — a task-level all covering a project-level blocking —
// two documents import cleanly under TP_UNATTENDED=1. Either is writable by the
// unit itself, which is what makes row 13b's refusal an authoring error rather
// than an escalation, and why tp escalate --decision audit-converge-on stays
// reserved for a unit that intends the relax.
//
// Row 13d's mutant is refusing whenever the imported block omits the field.
// That mutant refuses the first document here, and refusing it returns the
// deadlock the change rule was written to remove: the document with no
// top-level workflow key is the one tp import's preservation step is built
// around.
//
// The shell is the refusing one, asserted before each import rather than
// assumed. A remedy that passed over some other shell would not be a remedy for
// anything.
func TestAuditConvergeOnFence_ImportRemediesPass(t *testing.T) {
	t.Run("omitting the top-level workflow key", func(t *testing.T) {
		dir := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
		require.Equal(t, "all", fenceResolved(t, dir),
			"the task override covers the project block before the import")
		doc := fenceImportDoc(t, dir, "")

		_, stderr, code := runTPFence(t, dir, true, "import", doc)
		require.Equal(t, 0, code, "preservation carries the task-level all forward: %s", stderr)

		data, err := os.ReadFile(filepath.Join(dir, "s.tasks.json"))
		require.NoError(t, err)
		assert.Contains(t, string(data), `"audit_converge_on": "all"`,
			"the carried-forward block is what was written")
		assert.Equal(t, "all", fenceResolved(t, dir), "so nothing resolves differently")
	})

	t.Run("carrying audit_converge_on all explicitly", func(t *testing.T) {
		dir := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
		require.Equal(t, "all", fenceResolved(t, dir),
			"the task override covers the project block before the import")
		doc := fenceImportDoc(t, dir, `{"audit_converge_on":"all","review_max_rounds":5}`)

		_, stderr, code := runTPFence(t, dir, true, "import", doc)
		require.Equal(t, 0, code, "the document carries the resolved value itself: %s", stderr)

		data, err := os.ReadFile(filepath.Join(dir, "s.tasks.json"))
		require.NoError(t, err)
		assert.Contains(t, string(data), `"audit_converge_on": "all"`,
			"the document's own block replaced the old one and still covers the project layer")
		assert.Equal(t, "all", fenceResolved(t, dir), "so nothing resolves differently")
	})
}

// TestAuditConvergeOnFence_TheBlockTPWritesTakesTheRefusedPath pins the note
// §7 row 13d carries: model.TaskFile gives workflow no omitempty, so every file
// tp itself writes carries the top-level key, and a document built from one
// takes row 13b's path rather than row 13d's. The refusing input is therefore
// the common shape, not an exotic one — which is the whole reason the two
// remedies have to be reachable by a unit that never wrote a workflow block on
// purpose.
//
// The block is read out of the file tp init emits and fed straight back into
// the document, rather than restated as a literal here. A test that hard-coded
// {} would keep passing on the day the field gained an omitempty tag and the
// claim stopped being true.
func TestAuditConvergeOnFence_TheBlockTPWritesTakesTheRefusedPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.md"),
		[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))

	_, stderr, code := runTPFence(t, dir, true, "init", "s.md")
	require.Equal(t, 0, code, "init: %s", stderr)

	raw, err := os.ReadFile(filepath.Join(dir, "s.tasks.json"))
	require.NoError(t, err)
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keys))
	emitted, ok := keys["workflow"]
	require.True(t, ok,
		"workflow has no omitempty, so a file tp writes carries the top-level key")
	assert.JSONEq(t, "{}", string(emitted), "and a fresh init emits it empty")

	// That same block, imported over row 13b's shell, replaces the task-level
	// all and lets the project-level blocking resolve — naming neither literal.
	shell := fenceShell(t, `{"audit_converge_on":"all"}`, `{"audit_converge_on":"blocking"}`)
	require.Equal(t, "all", fenceResolved(t, shell),
		"the task override covers the project block before the import")
	doc := fenceImportDoc(t, shell, string(emitted))

	_, stderr, code = runTPFence(t, shell, true, "import", doc)
	assertFenceRefused(t, stderr, code, "a document carrying the block tp itself writes")
	assert.Equal(t, "all", fenceResolved(t, shell), "and the block stayed covered")
}

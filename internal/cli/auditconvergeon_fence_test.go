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

// TestAuditConvergeOnFence_ExtractPassesWhatPreservesResolution pins the fourth
// write path. tp config --extract runs §3's change rule over the population it
// already reads: each scanned task file's own override, resolved over the
// project block before and after the merge, with the hoisted fields stripped
// from the task layer exactly as the thinning strips them.
//
// This asserts the passing half only, and the reason is measured rather than
// reasoned — which is the mistake §3 records a draft of itself making at this
// very command. No input reaches the refusal at this sink today, for two
// independent reasons. hoistedFields does not yet carry audit_converge_on, so
// --extract cannot move the field at all; and — this one survives that landing —
// the scanned-file population is resolution-preserving by construction, since a
// task file whose blocking is hoisted into .tp/config.json and then stripped
// from its own block resolves blocking from the project layer afterwards exactly
// as it resolved it from the task layer before.
//
// Built and run rather than argued: with the field added to computeCommonPolicy,
// hoistedFields and mergeCommon in a copy of this tree, `TP_UNATTENDED=1 tp
// config --extract` over two task files carrying blocking exits 0 and hoists it,
// while a third spec with no task file at all moves from default/all to
// project/blocking. That third base is the refusing input, and widening the
// population to reach it is a separate piece of work.
func TestAuditConvergeOnFence_ExtractPassesWhatPreservesResolution(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	for _, base := range []string{"a", "b"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"),
			[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, base+".tasks.json"),
			[]byte(`{"version":1,"spec":"`+base+`.md","tasks":[],"workflow":`+
				`{"audit_converge_on":"blocking","review_max_rounds":7}}`), 0o600))
	}

	out, stderr, code := runTPFence(t, dir, true, "config", "--extract")
	require.Equal(t, 0, code, "the hoist preserves what every scanned file resolves: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Contains(t, res["hoisted"], "review_max_rounds",
		"the common field was hoisted, so the fence ran on a command that had work to do")

	data, err := os.ReadFile(filepath.Join(dir, "a.tasks.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"audit_converge_on": "blocking"`,
		"and the command completed rather than aborting part-written")
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

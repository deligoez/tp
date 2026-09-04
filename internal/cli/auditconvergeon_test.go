package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolvedAuditConvergeOn returns audit_converge_on's {value, source} object
// from tp config --resolved, run in dir. It takes no field name: every caller
// asks for this one field, and a parameter that only ever receives one literal
// is generality the gate's unparam linter rejects.
func resolvedAuditConvergeOn(t *testing.T, dir string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	wf, ok := res["workflow"].(map[string]any)
	require.True(t, ok, "config --resolved carries a workflow object")
	entry, ok := wf["audit_converge_on"].(map[string]any)
	require.True(t, ok, "config --resolved reports audit_converge_on")
	return entry
}

// TestAuditConvergeOnDefaultAll covers v0.37.0 §7 row 1: on a repo with no
// configuration at any layer, audit_converge_on resolves to all and tp config
// --resolved attributes it to the built-in default. §2 makes this asymmetry with
// review_converge_on deliberate, so the mutant that must fail this test is
// shipping blocking — the twin's default — as this field's built-in.
func TestAuditConvergeOnDefaultAll(t *testing.T) {
	t.Parallel()
	dir := writeStrategyProject(t, "{}")

	field := resolvedAuditConvergeOn(t, dir)
	assert.Equal(t, "all", field["value"], "the built-in default is all, not blocking")
	assert.Equal(t, "default", field["source"], "no layer sets it")
}

// TestAuditConvergeOnResolvedNamesItsSource covers v0.37.0 §2's resolution order
// at the reporting surface: a project-config value is attributed to project, and
// a task-file workflow block outranks it and is attributed to override. Neither
// literal is written through a set command, which §3 fences and a later task
// builds; both are stored values the parser must accept.
func TestAuditConvergeOnResolvedNamesItsSource(t *testing.T) {
	t.Parallel()
	dir := writeStrategyProject(t, "{}")
	tpDir := filepath.Join(dir, ".tp")
	require.NoError(t, os.Mkdir(tpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "config.json"),
		[]byte(`{"workflow":{"audit_converge_on":"blocking"}}`), 0o600))

	field := resolvedAuditConvergeOn(t, dir)
	assert.Equal(t, "blocking", field["value"], "the project default flows into resolution")
	assert.Equal(t, "project", field["source"], "the project layer is named")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[],"workflow":{"audit_converge_on":"all"}}`), 0o600))

	field = resolvedAuditConvergeOn(t, dir)
	assert.Equal(t, "all", field["value"], "the task override outranks the project config")
	assert.Equal(t, "override", field["source"], "the override layer is named")
}

// TestAuditConvergeOnHasNoEnvironmentLayer covers the environment half of
// v0.37.0 §7 row 2 through the whole read path rather than the resolver alone:
// a child process carrying TP_AUDIT_CONVERGE_ON=blocking still resolves all,
// and still attributes it to the built-in default. The value is the opposite of
// the built-in, so the assertion fails under the named mutant — an environment
// entry added to engine.ResolveWorkflowLayers at any rank answers this repo,
// which sets the field at no layer, with blocking.
func TestAuditConvergeOnHasNoEnvironmentLayer(t *testing.T) {
	t.Parallel()
	dir := writeStrategyProject(t, "{}")

	out, stderr, code := runTPEnv(t, dir, []string{"TP_AUDIT_CONVERGE_ON=blocking"}, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	wf, ok := res["workflow"].(map[string]any)
	require.True(t, ok, "config --resolved carries a workflow object")
	entry, ok := wf["audit_converge_on"].(map[string]any)
	require.True(t, ok, "config --resolved reports audit_converge_on")

	assert.Equal(t, "all", entry["value"], "an environment variable does not set the field")
	assert.Equal(t, "default", entry["source"], "and no layer is invented to attribute it to")
}

// TestAuditConvergeOnHasNoCommandFlag covers the flag half of v0.37.0 §7 row 2.
// There is no --audit-converge-on layer, so the observable fact is a refusal:
// every command that reads or writes the field must reject the flag as unknown
// rather than accept it silently. That refusal IS the assertion — adding the
// flag to any one of these commands reddens that subtest and leaves the others
// green.
func TestAuditConvergeOnHasNoCommandFlag(t *testing.T) {
	t.Parallel()
	dir := writeStrategyProject(t, "{}")

	for _, name := range []string{"audit", "config", "set", "run", "review"} {
		t.Run(name, func(t *testing.T) {
			_, stderr, code := runTP(t, dir, name, "--audit-converge-on=blocking")
			e := errJSON(t, stderr)
			assert.Equal(t, 2, code, "an unknown flag is a usage error")
			assert.Equal(t, float64(2), e["code"])
			msg, ok := e["error"].(string)
			require.True(t, ok, "the error object carries a message")
			assert.Contains(t, msg, "unknown flag: --audit-converge-on",
				"the flag is refused, not accepted as a resolution layer")
		})
	}
}

// TestAuditConvergeOn_InvalidStoredInTaskFileExitsValidation covers the consume
// half of v0.37.0 §7 row 3: an illegal audit_converge_on already stored in a
// task file — a hand edit, or a layer no write sink guards — makes both audit
// sinks exit ExitValidation (1) with the legal-values hint. The code is
// deliberately not ExitUsage (2): nothing on this invocation's command line is
// wrong, so it is a fault in the tree, not in the call. The mutant row 3 names
// is mapping both sinks to one code.
//
// The assertion on --record is not the exit code alone. Measured on the shipped
// binary before this test existed, `tp audit <spec> --record` over exactly this
// tree exited 0 and stored round 1, because the round is written before the
// workflow the payload reports is resolved. A refusal that lands after that
// write leaves a round recorded under a policy tp refused to read, in a store §2
// declares immutable — so the round file and the state index must both still be
// absent.
func TestAuditConvergeOn_InvalidStoredInTaskFileExitsValidation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","workflow":{"audit_converge_on":"bogus"},"tasks":[]}`), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	assert.Equal(t, 1, code, "--status exits ExitValidation on an illegal stored audit_converge_on")
	assert.Contains(t, stderr, "must be one of: blocking, all", "the refusal names the legal values")

	_, stderr, code = auditRecord(t, dir, `{"id":"a","status":"PASS"}`+"\n")
	assert.Equal(t, 1, code, "--record exits ExitValidation on an illegal stored audit_converge_on")
	assert.Contains(t, stderr, "must be one of: blocking, all", "the refusal names the legal values")

	stateDir := filepath.Join(dir, ".tp-review", "spec")
	for _, name := range []string{"audit-round-1.ndjson", "state.json"} {
		_, err := os.Stat(filepath.Join(stateDir, name))
		assert.True(t, os.IsNotExist(err),
			"a refused --record wrote no %s; the exit code alone cannot see this", name)
	}
}

// assertQuotesTheStoredValue asserts that an audit sink's error envelope names
// the illegal value it read and carries the shared legal-values hint. It reads
// the parsed envelope rather than the raw stderr: output.Error JSON-encodes the
// message, so a raw substring assertion for a quoted literal never matches and
// reddens on a refusal that is in fact correct.
func assertQuotesTheStoredValue(t *testing.T, stderr string) {
	t.Helper()
	e := errJSON(t, stderr)
	assert.Equal(t, float64(1), e["code"], "the envelope reports the validation code")
	assert.Equal(t, `invalid audit_converge_on value "bogus"`, e["error"],
		"the refusal quotes the value it read, so the operator can find it")
	assert.Equal(t, "must be one of: blocking, all", e["hint"],
		"and carries the same hint the review twin gives for its own field")
}

// TestAuditConvergeOn_InvalidStoredInProjectConfigExitsValidation is the same
// refusal reached from the other stored layer. It is a separate test rather
// than a subtest of a shared fixture because the two layers fail differently:
// a task-file value is read through the base tp resolves from the spec path,
// while .tp/config.json is found by walking up to the repository root, so a
// check keyed on the task file alone passes the sibling test and misses every
// project that sets the field once for the whole repo — the way a repo is
// expected to set it. The mutant is validating only the override layer.
func TestAuditConvergeOn_InvalidStoredInProjectConfigExitsValidation(t *testing.T) {
	t.Parallel()
	dir := writeStrategyProject(t, "{}")
	tpDir := filepath.Join(dir, ".tp")
	require.NoError(t, os.Mkdir(tpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "config.json"),
		[]byte(`{"workflow":{"audit_converge_on":"bogus"}}`), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	assert.Equal(t, 1, code, "--status exits ExitValidation on an illegal project-config value")
	assertQuotesTheStoredValue(t, stderr)

	_, stderr, code = auditRecord(t, dir, `{"id":"a","status":"PASS"}`+"\n")
	assert.Equal(t, 1, code, "--record exits ExitValidation on an illegal project-config value")
	assertQuotesTheStoredValue(t, stderr)

	stateDir := filepath.Join(dir, ".tp-review", "spec")
	_, err := os.Stat(stateDir)
	assert.True(t, os.IsNotExist(err), "a refused --record created no state directory at all")
}

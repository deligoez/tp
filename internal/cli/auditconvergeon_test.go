package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolvedWorkflowField returns one workflow field's {value, source} object from
// tp config --resolved, run in dir.
func resolvedWorkflowField(t *testing.T, dir, field string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	wf, ok := res["workflow"].(map[string]any)
	require.True(t, ok, "config --resolved carries a workflow object")
	entry, ok := wf[field].(map[string]any)
	require.True(t, ok, "config --resolved reports %s", field)
	return entry
}

// TestAuditConvergeOnDefaultAll covers v0.37.0 §7 row 1: on a repo with no
// configuration at any layer, audit_converge_on resolves to all and tp config
// --resolved attributes it to the built-in default. §2 makes this asymmetry with
// review_converge_on deliberate, so the mutant that must fail this test is
// shipping blocking — the twin's default — as this field's built-in.
func TestAuditConvergeOnDefaultAll(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	field := resolvedWorkflowField(t, dir, "audit_converge_on")
	assert.Equal(t, "all", field["value"], "the built-in default is all, not blocking")
	assert.Equal(t, "default", field["source"], "no layer sets it")
}

// TestAuditConvergeOnResolvedNamesItsSource covers v0.37.0 §2's resolution order
// at the reporting surface: a project-config value is attributed to project, and
// a task-file workflow block outranks it and is attributed to override. Neither
// literal is written through a set command, which §3 fences and a later task
// builds; both are stored values the parser must accept.
func TestAuditConvergeOnResolvedNamesItsSource(t *testing.T) {
	dir := writeStrategyProject(t, "{}")
	tpDir := filepath.Join(dir, ".tp")
	require.NoError(t, os.Mkdir(tpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "config.json"),
		[]byte(`{"workflow":{"audit_converge_on":"blocking"}}`), 0o600))

	field := resolvedWorkflowField(t, dir, "audit_converge_on")
	assert.Equal(t, "blocking", field["value"], "the project default flows into resolution")
	assert.Equal(t, "project", field["source"], "the project layer is named")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[],"workflow":{"audit_converge_on":"all"}}`), 0o600))

	field = resolvedWorkflowField(t, dir, "audit_converge_on")
	assert.Equal(t, "all", field["value"], "the task override outranks the project config")
	assert.Equal(t, "override", field["source"], "the override layer is named")
}

package cli_test

import (
	"encoding/json"
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

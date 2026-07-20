package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStatus_EmptyStateFullShape(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "empty state exits 0: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, []any{}, out["review_rounds"])
	assert.Equal(t, float64(0), out["consecutive_clean"])
	assert.Equal(t, float64(2), out["required_clean_rounds"])
	assert.Equal(t, false, out["converged"])
	assert.Equal(t, false, out["stale"])
	assert.Equal(t, []any{}, out["mechanical_checks"])
	_, hasBudget := out["budget_exhausted"]
	assert.False(t, hasBudget, "budget_exhausted only when a cap is set")
}

func TestReviewStatus_RoundsAndConvergence(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	_, _, code := recordRound(t, dir, "")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["converged"], "two clean rounds with default 2 required")
	rounds := out["review_rounds"].([]any)
	assert.Len(t, rounds, 2)

	// Editing the spec flips stale and unconverges
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec edited\n"), 0o600))
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "--status alone stays exit 0")
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["stale"])
	assert.Equal(t, false, out["converged"])
}

func TestReviewStatus_CheckRunsChecksAndGatesExit(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	// task file adjacent to spec so workflow resolution finds the checks
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", `checks=[{"class":"always-pass","cmd":"true"},{"class":"always-fail","cmd":"echo bad; exit 1"}]`)
	require.Equal(t, 0, code)

	// converged state: two clean rounds
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code, "failing check gates exit 1 despite convergence")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	checks := out["mechanical_checks"].([]any)
	require.Len(t, checks, 2)
	first := checks[0].(map[string]any)
	assert.Equal(t, true, first["passed"])
	_, hasTail := first["output_tail"]
	assert.False(t, hasTail, "output_tail only for failed checks")
	second := checks[1].(map[string]any)
	assert.Equal(t, false, second["passed"])
	assert.Contains(t, second["output_tail"], "bad")

	// Without --check: registered list only, no execution fields
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	checks = out["mechanical_checks"].([]any)
	require.Len(t, checks, 2)
	_, hasPassed := checks[0].(map[string]any)["passed"]
	assert.False(t, hasPassed, "no execution results without --check")
}

func TestReviewStatus_FlagRules(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	_, _, code := runTP(t, dir, "review", "spec.md", "--check")
	assert.Equal(t, 2, code, "bare --check is a usage error")

	_, _, code = runTP(t, dir, "review", "spec.md", "--record", "x.ndjson", "--check")
	assert.Equal(t, 2, code, "--record --check is a usage error")

	_, _, code = runTP(t, dir, "review", "spec.md", "--status", "--record", "x.ndjson")
	assert.Equal(t, 2, code, "--status and --record are mutually exclusive")

	_, _, code = runTP(t, dir, "review", "spec.md", "--status", "--spec-inline")
	assert.Equal(t, 2, code, "--status rejects prompt-generation flags")
}

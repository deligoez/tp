package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoundBudget_Escalates: review_max_rounds=2 and audit_max_rounds=2, each
// with 2 dirty rounds recorded — generation and further records are refused
// with the escalation hint, --status reports budget_exhausted, and import
// enforcement is unchanged by the cap.
func TestRoundBudget_Escalates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", "review_max_rounds=2", "audit_max_rounds=2")
	require.Equal(t, 0, code)

	for i := 0; i < 2; i++ {
		_, _, rc := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, rc)
		_, _, rc = auditRecord(t, dir, `{"id":"x","status":"FAIL"}`+"\n")
		require.Equal(t, 0, rc)
	}

	// Review prompt generation exits 4 with the escalation hint
	_, stderr, code := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "raise the cap")

	// Further --record beyond the exhausted cap is refused (review and audit)
	_, stderr, code = recordRound(t, dir, "")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "budget exhausted")
	_, stderr, code = auditRecord(t, dir, "")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "budget exhausted")

	// --status reports budget_exhausted: true on both shapes
	for _, cmd := range [][]string{
		{"review", "spec.md", "--status"},
		{"audit", "spec.md", "--status"},
	} {
		stdout, _, rc := runTP(t, dir, cmd...)
		require.Equal(t, 0, rc)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		assert.Equal(t, true, out["budget_exhausted"], "%v", cmd)
	}

	// audit_max_rounds blocks audit prompt generation identically
	_, stderr, code = runTP(t, dir, "audit", "spec.md")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "audit round budget exhausted")

	// Import enforcement unchanged: unconverged spec stays blocked (exit 1,
	// not relaxed by the cap)
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(`[`+enforceTask+`]`), 0o600))
	_, stderr, code = runTP(t, dir, "import", importPath, "--spec", "spec.md")
	assert.Equal(t, 1, code, "cap never changes import enforcement")
	assert.Contains(t, stderr, "review not converged")
}

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
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", "review_max_rounds=2", "audit_max_rounds=2")
	require.Equal(t, 0, code)

	for range 2 {
		_, _, rc := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, rc)
		_, _, rc = auditRecord(t, dir, `{"id":"x","status":"FAIL"}`+"\n")
		require.Equal(t, 0, rc)
	}

	// Review prompt generation exits 4, and the hint names the way out at the
	// cap: disposition the remaining findings.
	_, stderr, code := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "--resolve")

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

	// At the cap import passes only once every finding of the latest round
	// carries a disposition; an open one still blocks (exit 1).
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(`[`+enforceTask+`]`), 0o600))
	_, stderr, code = runTP(t, dir, "import", importPath, "--spec", "spec.md")
	assert.Equal(t, 1, code, "an undispositioned finding at the cap blocks import")
	assert.Contains(t, stderr, "carrying no disposition")
}

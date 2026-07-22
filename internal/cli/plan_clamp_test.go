package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlan_ClampsOutOfRangeWorkflow: tp plan resolves its workflow block with the
// same clamp as tp config/done/review, so an authored out-of-range value is
// reported as its clamped effective value, not verbatim (§3.5/§7.1 consistency).
func TestPlan_ClampsOutOfRangeWorkflow(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"version":1,"spec":"s.md","workflow":{"review_clean_rounds":0},"tasks":[{"id":"t1","title":"T","status":"open","estimate_minutes":5,"acceptance":"ok","source_sections":["## S"]}]}`), 0o600))

	stdout, stderr, code := runTP(t, dir, "plan")
	require.Equal(t, 0, code, "plan failed: %s", stderr)

	var plan map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	wf := plan["workflow"].(map[string]any)
	assert.Equal(t, float64(2), wf["review_clean_rounds"], "an out-of-range value is clamped to the built-in default in tp plan, matching other commands")
}

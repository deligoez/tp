package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// securityRows renders one PASS row per file_check item of the security
// prompt for the first n items, plus the first item a second time as FAIL.
func securityRows(t *testing.T, sec map[string]any, n int) string {
	t.Helper()
	items := sec["checklist_items"].([]any)
	require.GreaterOrEqual(t, len(items), n)
	var b strings.Builder
	row := func(id, status, sev string) {
		fmt.Fprintf(&b, `{"role":"security","item_id":%q,"status":%q,"evidence_file":"f.go","evidence_lines":"1","category":%s,"severity":%s,"notes":"","location":"§1","class":"x"}`+"\n",
			id, status, map[bool]string{true: "null", false: `"correctness"`}[status == "PASS"], sev)
	}
	for i := range n {
		row(items[i].(map[string]any)["item_id"].(string), "PASS", "null")
	}
	row(items[0].(map[string]any)["item_id"].(string), "FAIL", `"error"`)
	return b.String()
}

// TestAuditGraded_RecordStatusAndNextActionSayNOfM: decision 4. The round
// emitted over fourteen changed files gives each code role ten of them; the
// security role returns rows for four, one twice, and the other code role
// returns nothing. --record stores "graded 4 of 14" and "0 of 14" in the
// round's entry, and --record, --status and next_action report it.
func TestAuditGraded_RecordStatusAndNextActionSayNOfM(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 14)

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	pools := engine.AuditPoolsPath(specPath, 1)
	assert.FileExists(t, pools, "the emission keeps the round's pools in its inbox")

	results := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(securityRows(t, auditPromptsByRole(t, stdout)["security"], 4)), 0o600))

	stdout, stderr, code = runTP(t, dir, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rec))
	want := []any{
		map[string]any{"role": "security", "graded": float64(4), "of": float64(14)},
		map[string]any{"role": "maintainability-conventions", "graded": float64(0), "of": float64(14)},
	}
	assert.Equal(t, want, rec["graded"], "--record reports graded N of M per code role")
	assert.Contains(t, rec["next_action"], "(latest round graded security 4 of 14, maintainability-conventions 0 of 14)")
	assert.NoFileExists(t, pools, "the pools leave with the recorded round's inbox")

	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	require.Len(t, st.AuditRounds, 1)
	assert.Equal(t, []engine.RoleGraded{{Role: "security", Graded: 4, Of: 14}, {Role: "maintainability-conventions", Graded: 0, Of: 14}},
		st.AuditRounds[0].Graded, "the counts land in the round's entry in state.json")

	stdout, stderr, code = runTP(t, dir, "audit", specPath, "--status", "--compact")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	round := status["audit_rounds"].([]any)[0].(map[string]any)
	assert.Equal(t, want, round["graded"], "--status reads them from the state file")
	assert.Contains(t, status["next_action"], "graded security 4 of 14")
}

// TestAuditGraded_RecordWithoutEmissionStoresNone: a round recorded with no
// emission has no pools to grade against, so its entry carries no graded field
// and next_action stays as it was.
func TestAuditGraded_RecordWithoutEmissionStoresNone(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 3)
	results := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(`{"role":"security","item_id":"file-security-x.0000000000000000","status":"PASS","category":null,"severity":null,"notes":""}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rec))
	assert.NotContains(t, rec, "graded")
	assert.NotContains(t, rec["next_action"], "graded")
}

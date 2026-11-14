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
	// Well-formed ids the round never emitted: graded must not count them.
	row("file-security-not-emitted-go.00000000000000ff", "PASS", "null")
	row("file-security-another-go.00000000000000fe", "PASS", "null")
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
		map[string]any{"role": "spec-coverage", "graded": float64(0), "pool": float64(1)},
		map[string]any{"role": "security", "graded": float64(4), "pool": float64(14)},
		map[string]any{"role": "maintainability-conventions", "graded": float64(0), "pool": float64(14)},
	}
	assert.Equal(t, want, rec["graded"], "--record reports graded N of M per code role and for spec-coverage")
	next := rec["next_action"].(string)
	assert.True(t, strings.HasPrefix(next, "latest round graded spec-coverage 0 of 1, security 4 of 14, maintainability-conventions 0 of 14. "),
		"the graded clause leads next_action, apart from the step: %q", next)
	assert.True(t, strings.HasSuffix(next, "--resolve <role:item_id> wontfix|duplicate \"<evidence>\" (the operator's decision)") ||
		strings.HasSuffix(next, "--record <file>"), "and the step ends it unchanged, so copying the command copies only the command: %q", next)
	assert.NoFileExists(t, pools, "the pools leave with the recorded round's inbox")

	gradedEntries := []engine.RoleGraded{{Role: "spec-coverage", Graded: 0, Pool: 1}, {Role: "security", Graded: 4, Pool: 14}, {Role: "maintainability-conventions", Graded: 0, Pool: 14}}
	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	require.Len(t, st.AuditRounds, 1)
	assert.Equal(t, gradedEntries, st.AuditRounds[0].Graded, "the counts land in the round's entry in state.json")

	stdout, stderr, code = runTP(t, dir, "audit", specPath, "--status", "--compact")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	round := status["audit_rounds"].([]any)[0].(map[string]any)
	assert.Equal(t, want, round["graded"], "--status reads them from the state file")
	assert.True(t, strings.HasPrefix(status["next_action"].(string), "latest round graded spec-coverage 0 of 1, security 4 of 14"), "--status leads with it too")
}

// TestAuditGraded_RecordWithoutEmissionStoresNone: a round recorded with no
// emission has no pools to grade against, so its entry carries no graded field
// and next_action stays as it was; since v1.20.0 it is stamped unasked and is
// not clean, since nothing shows what spec-coverage was asked
// (spec/backlog/role-presence.md §2.3).
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
	assert.Equal(t, false, rec["clean"], "an all-PASS round with no emission is not clean")
	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	assert.True(t, st.AuditRounds[0].Unasked)
}

// TestAuditGraded_SpecCoverageMustAnswerEveryAskedItem: an all-PASS round is
// clean only when spec-coverage returned a row for every checklist item its
// emission asked (spec/backlog/role-presence.md §2.2).
func TestAuditGraded_SpecCoverageMustAnswerEveryAskedItem(t *testing.T) {
	t.Parallel()
	for _, answered := range []bool{false, true} {
		dir, specPath := auditRepoWithChangedFiles(t, 2)
		stdout, stderr, code := runTP(t, dir, "audit", specPath)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var b strings.Builder
		if answered {
			for _, it := range auditPromptsByRole(t, stdout)["spec-coverage"]["checklist_items"].([]any) {
				fmt.Fprintf(&b, `{"role":"spec-coverage","item_id":%q,"status":"PASS","category":null,"severity":null,"notes":""}`+"\n", it.(map[string]any)["item_id"])
			}
		}
		results := filepath.Join(dir, "results.ndjson")
		require.NoError(t, os.WriteFile(results, []byte(b.String()), 0o600))

		stdout, stderr, code = runTP(t, dir, "audit", specPath, "--record", results)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var rec map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &rec))
		assert.Equal(t, answered, rec["clean"], "spec-coverage answered every asked item: %v", answered)
	}
}

// TestAuditGraded_ReRecordKeepsWhatWasGraded: a unit's re-record of its own
// round (TP_ROUND, the idempotent retry) finds no pools, since the first record
// removed them, and must keep the graded the first record stored rather than
// wipe it.
func TestAuditGraded_ReRecordKeepsWhatWasGraded(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 14)
	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	results := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(securityRows(t, auditPromptsByRole(t, stdout)["security"], 4)), 0o600))

	_, stderr, code = runTPEnv(t, dir, []string{"TP_ROUND=1"}, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = runTPEnv(t, dir, []string{"TP_ROUND=1"}, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	require.Len(t, st.AuditRounds, 1, "the retry rewrote round 1")
	assert.Equal(t, []engine.RoleGraded{{Role: "spec-coverage", Graded: 0, Pool: 1}, {Role: "security", Graded: 4, Pool: 14}, {Role: "maintainability-conventions", Graded: 0, Pool: 14}},
		st.AuditRounds[0].Graded, "the re-record keeps what the first record graded")
}

// TestAuditGraded_StrayPoolsGradeNoNoEmissionRecord: a round recorded with no
// emission of its own (hash_scheme no-emission) answers for no reading, so a
// pools file left at its number does not give it a graded.
func TestAuditGraded_StrayPoolsGradeNoNoEmissionRecord(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 3)
	require.NoError(t, engine.WriteAuditPools(specPath, 1, []engine.AuditRolePool{
		{Role: "security", Pool: 3, Items: []string{"file-security-x.0000000000000000"}},
	}))
	results := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(`{"role":"security","item_id":"file-security-x.0000000000000000","status":"PASS","category":null,"severity":null,"notes":""}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rec))
	assert.NotContains(t, rec, "graded", "no emission, no graded, whatever sits in the inbox")
	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	require.Len(t, st.AuditRounds, 1)
	assert.Equal(t, engine.HashSchemeNoEmission, st.AuditRounds[0].HashScheme)
	assert.Nil(t, st.AuditRounds[0].Graded)
}

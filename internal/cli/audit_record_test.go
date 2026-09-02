package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// The run goes through runTPFence with unattended false rather than through
// runTP, so TP_UNATTENDED is decided here instead of inherited: runTP builds
// every child from os.Environ(), and under tp run the quality gate itself runs
// in a child carrying TP_UNATTENDED=1. No caller of this helper is about the
// unattended fence, so the variable is pinned off for all of them. Measured
// with a probe that exits 2 from runAuditRecord under the variable: with it
// inherited, four tests in audit_stamping_test.go and
// audit_nextaction_count_test.go fail under an ambient TP_UNATTENDED=1; pinned,
// they pass either way.
func auditRecord(t *testing.T, dir, ndjsonContent string) (out map[string]any, stderr string, code int) {
	t.Helper()
	f := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(ndjsonContent), 0o600))
	var stdout string
	stdout, stderr, code = runTPFence(t, dir, false, "audit", "spec.md", "--record", f)
	if code == 0 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	}
	return out, stderr, code
}

func TestAuditRecord_CountsNonPass(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// PASS / PARTIAL / FAIL / absent -> 3 findings
	out, stderr, code := auditRecord(t, dir,
		`{"id":"a","status":"PASS"}`+"\n"+
			`{"id":"b","status":"PARTIAL"}`+"\n"+
			`{"id":"c","status":"FAIL"}`+"\n"+
			`{"id":"d"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(3), out["findings"])
	assert.Equal(t, false, out["clean"])
	_, hasCandidates := out["mechanize_candidates"]
	assert.False(t, hasCandidates, "audit output has no mechanize_candidates")

	// All-PASS round is clean; audit sequence appends
	out, _, code = auditRecord(t, dir, `{"id":"a","status":"PASS"}`+"\n")
	require.Equal(t, 0, code)
	assert.Equal(t, float64(2), out["round"])
	assert.Equal(t, true, out["clean"])

	// Round files land in state.json.audit_rounds
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st struct {
		ReviewRounds []any `json:"review_rounds"`
		AuditRounds  []any `json:"audit_rounds"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	assert.Len(t, st.AuditRounds, 2)
	assert.Empty(t, st.ReviewRounds, "audit sequence is independent of review rounds")
}

// TestAuditRecord_StoredFindingsFollowTheSharedPredicate is the observable form
// of a claim that used to be asserted against a copy of the implementation.
// engine's TestAuditRowIsPass_MatchesRecordPathFindingCount counted the same
// rows with a hand-written `status, _ := row["status"].(string); status !=
// "PASS"` labelled "the record path's literal test" and compared that copy with
// engine.AuditRowIsPass. countAuditFindings does not restate the literal — it
// calls the predicate — so the copy pinned nothing on the record path: a record
// path that trimmed `status` by symmetry with `role` left that test green.
//
// The number this guards is the round's stored `findings`, which decides
// `clean`, the streak and the convergence gate. The rows are the ones that
// separate the shared predicate from any restatement of it: " PASS " and
// "pass" are non-PASS, a non-string status is non-PASS, and an absent key is
// non-PASS.
func TestAuditRecord_StoredFindingsFollowTheSharedPredicate(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	rows := []map[string]any{
		{"id": "a", "status": "PASS"},
		{"id": "b", "status": "pass"},
		{"id": "c", "status": " PASS "},
		{"id": "d", "status": float64(1)},
		{"id": "e"},
		{"id": "f", "status": "FAIL"},
	}
	lines := make([]string, 0, len(rows))
	want := 0
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		require.NoError(t, err)
		lines = append(lines, string(encoded))
		if !engine.AuditRowIsPass(row) {
			want++
		}
	}
	require.Equal(t, 5, want, "exactly one of the six rows is PASS by the shared predicate")

	out, stderr, code := auditRecord(t, dir, strings.Join(lines, "\n")+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, float64(want), out["findings"],
		"the emitted count is the shared predicate's, not a restatement that trims or folds case")
	assert.Equal(t, false, out["clean"])

	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	require.Len(t, st.AuditRounds, 1)
	assert.Equal(t, want, st.AuditRounds[0].Findings,
		"and the same number is what state.json stores for the round")
}

func TestAuditStatus_Shapes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// Empty-state shape, exit 0
	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, []any{}, out["audit_rounds"])
	assert.Equal(t, float64(0), out["consecutive_clean"])
	assert.Equal(t, float64(2), out["required_clean_rounds"])
	assert.Equal(t, false, out["converged"])
	assert.Equal(t, false, out["stale"])
	_, hasMech := out["mechanical_checks"]
	assert.False(t, hasMech, "audit status has no mechanical_checks")

	// Two clean rounds -> converged; --check exits 0
	_, _, code = auditRecord(t, dir, "")
	require.Equal(t, 0, code)
	_, _, code = auditRecord(t, dir, "")
	require.Equal(t, 0, code)

	_, _, code = runTP(t, dir, "audit", "spec.md", "--status", "--check")
	assert.Equal(t, 0, code, "converged audit passes --check")

	// Editing the spec flips stale; --check exits 1
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec edited\n"), 0o600))
	_, _, code = runTP(t, dir, "audit", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code, "stale audit fails --check")
}

func TestAuditRecordStatus_FlagRejections(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	cases := [][]string{
		{"audit", "spec.md", "--record", "x.ndjson", "--affected-files", "a.go"},
		{"audit", "spec.md", "--record", "x.ndjson", "--findings", "f.ndjson"},
		{"audit", "spec.md", "--status", "--affected-files", "a.go"},
		{"audit", "spec.md", "--status", "--findings", "f.ndjson"},
		{"audit", "spec.md", "--record", "x.ndjson", "--status"},
		{"audit", "spec.md", "--check"},
	}
	for _, args := range cases {
		_, _, code := runTP(t, dir, args...)
		assert.Equal(t, 2, code, "args %v must be a usage error", args)
	}
}

func TestAuditRecord_RoleMissingWarns(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// Line 1 omits `role`; line 2 carries it. The round still records (exit 0)
	// and the warning names the offending line and the results file.
	ndjson := `{"id":"a","status":"FAIL","item_id":"i1"}` + "\n" +
		`{"id":"b","status":"FAIL","item_id":"i2","role":"go-safety"}` + "\n"
	out, stderr, code := auditRecord(t, dir, ndjson)
	require.Equal(t, 0, code, "round still records despite the role-less row")
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(2), out["findings"])
	assert.Contains(t, stderr, "missing the role field")
	assert.Contains(t, stderr, "line 1")
	assert.Contains(t, stderr, "results.ndjson", "warning must name the results file")
	assert.NotContains(t, stderr, "line 2", "a row carrying role must not warn")
}

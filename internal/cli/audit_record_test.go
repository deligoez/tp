package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func auditRecord(t *testing.T, dir, ndjsonContent string) (out map[string]any, stderr string, code int) {
	t.Helper()
	f := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(ndjsonContent), 0o600))
	var stdout string
	stdout, stderr, code = runTP(t, dir, "audit", "spec.md", "--record", f)
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

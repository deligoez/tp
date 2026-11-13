package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The BUGS.md audit row: one error-severity FAIL nobody re-grades or accepts.
const auditCarriedFail = `{"role":"spec-coverage","item_id":"§2","status":"FAIL","severity":"error","category":"correctness","notes":"rule 1 is not implemented"}` + "\n"

// auditCarriedRepro is the BUGS.md reproduction: round 1 records the FAIL, and
// rounds 2 and 3 record nothing, the spec unchanged throughout. Two clean
// rounds reach the default audit_clean_rounds, and three the default cap.
func auditCarriedRepro(t *testing.T, rounds ...string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(carriedSpec), 0o600))
	for _, rows := range rounds {
		_, stderr, code := auditRecord(t, dir, rows)
		require.Equal(t, 0, code, "record: %s", stderr)
	}
	return dir
}

// auditStatusCheck runs tp audit spec.md --status --check and returns its
// payload and exit code.
func auditStatusCheck(t *testing.T, dir string) (out map[string]any, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--status", "--check")
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "status payload: %s", stderr)
	return out, code
}

// TestAuditCarriedBlocking_EmptyRoundsDoNotConvergePastAFail is the BUGS.md
// repro: the FAIL of round 1 is asked to be re-graded, the rounds after it
// record nothing, and the loop is not converged while it stands. --status
// names the row and how it clears.
func TestAuditCarriedBlocking_EmptyRoundsDoNotConvergePastAFail(t *testing.T) {
	t.Parallel()
	dir := auditCarriedRepro(t, auditCarriedFail, "", "")

	out, code := auditStatusCheck(t, dir)
	assert.Equal(t, 1, code, "--check exits 1 while a carried blocking FAIL stands")
	assert.Equal(t, false, out["converged"])
	assert.Equal(t, false, out["done"])
	assert.Equal(t, float64(1), out["carried_blocking"])
	rows := carriedRows(t, out)
	require.Len(t, rows, 1)
	assert.Equal(t, float64(1), rows[0]["round"])
	assert.Equal(t, float64(0), rows[0]["index"])
	assert.Equal(t, "spec-coverage", rows[0]["role"])
	assert.Equal(t, "§2", rows[0]["item_id"])
	assert.Equal(t, "error", rows[0]["severity"])
	assert.Equal(t, ".tp-review/spec/audit-round-1.ndjson", rows[0]["file"])
}

// TestAuditCarriedBlocking_EveryLaterPromptAsksForTheReGrade: a FAIL only a
// re-grade clears must reach a role in every later round, not only the one
// after it. Round 2 records nothing; round 3's security prompt still lists
// round 1's row, marked with the round it was graded in.
func TestAuditCarriedBlocking_EveryLaterPromptAsksForTheReGrade(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	for _, rows := range []string{`{"item_id":"carried-sec","status":"FAIL","role":"security"}` + "\n", ""} {
		_, stderr, code := auditRecord(t, dir, rows)
		require.Equal(t, 0, code, "record: %s", stderr)
	}

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]["prompt"].(string)
	assert.Contains(t, sec, "## Prior Round")
	assert.Contains(t, sec, `{"role":"security","item_id":"carried-sec","status":"FAIL","round":1}`)
	assert.Contains(t, sec, "graded in that earlier round")
}

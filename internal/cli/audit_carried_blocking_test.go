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

// TestAuditCarriedBlocking_NextActionNamesTheReGradeAndTheExits: below the cap
// next_action names the carried row and the round that re-grades it, beside the
// operator's acceptance; at the cap, where no round is admitted, it names the
// acceptance and the cap's raise, both the operator's decisions.
func TestAuditCarriedBlocking_NextActionNamesTheReGradeAndTheExits(t *testing.T) {
	t.Parallel()
	below, _ := auditStatusCheck(t, auditCarriedRepro(t, auditCarriedFail, ""))
	next, _ := below["next_action"].(string)
	assert.Contains(t, next, "1 carried blocking audit row")
	assert.Contains(t, next, "re-grade")
	assert.Contains(t, next, "tp audit spec.md --record <file>")
	assert.Contains(t, next, "--resolve spec-coverage:§2 wontfix|duplicate")

	atCap, _ := auditStatusCheck(t, auditCarriedRepro(t, auditCarriedFail, "", ""))
	next, _ = atCap["next_action"].(string)
	assert.Contains(t, next, "--resolve spec-coverage:§2 wontfix|duplicate")
	assert.Contains(t, next, "raise-audit-cap")
	assert.NotContains(t, next, "proceed to release")
}

// TestAuditCarriedBlocking_TheCapRefusesAFourthRound: at the cap the empty
// rounds no longer count as convergence, so a fourth emission is refused, and
// its hint names the carried row and the operator's exits.
func TestAuditCarriedBlocking_TheCapRefusesAFourthRound(t *testing.T) {
	t.Parallel()
	dir := auditCarriedRepro(t, auditCarriedFail, "", "")
	_, stderr, code := runTP(t, dir, "audit", "spec.md")
	assert.Equal(t, 4, code, "the cap refuses a fourth round: %s", stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "round budget exhausted")
	assert.Contains(t, env["hint"], "--resolve spec-coverage:§2 wontfix|duplicate")
	assert.Contains(t, env["hint"], "raise-audit-cap")
}

// TestAuditCarriedBlocking_AnEpochPastABlockingAuditRowIsTheOperators: an epoch
// whose audit cut holds an unresolved blocking audit row stops it blocking
// with neither a re-grade nor an acceptance, so it is rewrite-past-blocking as
// for a review row: refused under TP_UNATTENDED, and by hand it names the cut.
func TestAuditCarriedBlocking_AnEpochPastABlockingAuditRowIsTheOperators(t *testing.T) {
	t.Parallel()
	dir := auditCarriedRepro(t, auditCarriedFail)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(carriedSpec+"\n## 3. Added\n\nA new rule.\n"), 0o600))

	_, stderr, code := runTPFence(t, dir, true, "review", "spec.md", "--rewritten", "replaced the spec")
	assert.Equal(t, 2, code, stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "unresolved blocking")
	assert.Contains(t, env["hint"], "--resolve spec-coverage:§2")
	assert.Contains(t, env["hint"], "tp escalate --decision rewrite-past-blocking")

	stdout, stderr, code := runTPFence(t, dir, false, "review", "spec.md", "--rewritten", "replaced the spec")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "rewrite-past-blocking")
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["cut_blocking"])
	cut := carriedRows(t, map[string]any{"carried_blocking_rows": out["cut_blocking_rows"]})
	require.Len(t, cut, 1)
	assert.Equal(t, "§2", cut[0]["item_id"])
}

// TestAuditCarriedBlocking_ExportCountsTheCarriedRows: tp export's audit loop
// counts the carried blocking rows holding it open, as its review loop does.
func TestAuditCarriedBlocking_ExportCountsTheCarriedRows(t *testing.T) {
	t.Parallel()
	dir := auditCarriedRepro(t, auditCarriedFail, "")
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code := runTP(t, dir, "export")
	require.Equal(t, 0, code, stderr)
	var out struct {
		Loops struct {
			Audit map[string]any `json:"audit"`
		} `json:"loops"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out.Loops.Audit["carried_blocking"])
}

// TestAuditCarriedBlocking_ARowNoRoleCanReGradeSaysSo: a carried row whose role
// the audit panel no longer holds reaches no prompt, so no round re-grades it:
// next_action says so and names only the operator's exits.
func TestAuditCarriedBlocking_ARowNoRoleCanReGradeSaysSo(t *testing.T) {
	t.Parallel()
	retired := `{"role":"retired-role","item_id":"task-t1","status":"FAIL","severity":"error","category":"correctness","notes":"n"}` + "\n"
	out, _ := auditStatusCheck(t, auditCarriedRepro(t, retired, ""))
	next, _ := out["next_action"].(string)
	assert.Contains(t, next, "no role of the next round can re-grade it")
	assert.Contains(t, next, "--resolve retired-role:task-t1 wontfix|duplicate")
	assert.NotContains(t, next, "run it — tp audit spec.md --record <file>")
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

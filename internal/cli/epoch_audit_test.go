package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditRewrittenFixture records audit round 1 with one security FAIL whose
// item id is a marker, then rewrites the spec.
func auditRewrittenFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, stderr, code := auditRecord(t, dir,
		`{"item_id":"MARKER-AUDIT-OLD","status":"FAIL","role":"security","evidence_file":"auth_helper.go"}`+"\n")
	require.Equal(t, 0, code, "record audit round 1: %s", stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec+"\n## Appendix\n\nRewritten.\n"), 0o600))
	return dir
}

// TestEpoch_TheAuditPriorRoundSectionStartsAtTheEpoch: the round-2 audit
// prompt's prior-round section reads the last recorded audit round; one
// recorded before the epoch graded a text the spec no longer holds, so after
// the epoch no prompt carries the section, and without it the prompts do.
func TestEpoch_TheAuditPriorRoundSectionStartsAtTheEpoch(t *testing.T) {
	t.Parallel()
	without := auditRewrittenFixture(t)
	stdout, stderr, code := runTP(t, without, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, stderr)
	require.Contains(t, auditPromptsByRole(t, stdout)["security"]["prompt"], "MARKER-AUDIT-OLD",
		"without an epoch round 2 carries round 1's FAIL")

	dir := auditRewrittenFixture(t)
	startEpoch(t, dir)
	stdout, stderr, code = runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, stderr)
	for role, p := range auditPromptsByRole(t, stdout) {
		assert.NotContains(t, p["prompt"], "## Prior Round", "%s: no prior-round section after the epoch", role)
		assert.NotContains(t, p["prompt"], "MARKER-AUDIT-OLD", role)
	}
}

// TestEpoch_AnAcceptanceFromBeforeTheEpochIsNotCarried: --record carries an
// acceptance from the round immediately before; one decided before the epoch
// was made about a text the spec no longer holds, so it is not carried and
// the operator is asked for it again. carryRepo's fixture carries it without
// the epoch (TestAuditCarry_AnUnchangedAcceptanceIsCarriedIntoTheNextRound).
func TestEpoch_AnAcceptanceFromBeforeTheEpochIsNotCarried(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2021-01-01T00:00:00Z", carryFailRow, carryWontfix)
	statePath := filepath.Join(dir, ".tp-review", "spec", "state.json")
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)
	withEpoch := strings.TrimSuffix(strings.TrimSpace(string(data)), "}") +
		`,"epochs":[{"after_review_round":0,"after_audit_round":1,"evidence":"rewritten","at":"2021-01-02T00:00:00Z"}]}`
	require.NoError(t, os.WriteFile(statePath, []byte(withEpoch), 0o600))

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, float64(0), out["carried"], "nothing is carried across the epoch")
	assert.Equal(t, false, out["clean"], "the FAIL stands until the operator decides it again")
	assert.NotContains(t, carriedRow(t, dir, 2), "resolved")
}

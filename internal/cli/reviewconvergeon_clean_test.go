package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupConvergeOnProject writes a spec, inits its task file, and sets
// review_clean_rounds=1 so a single clean round converges.
func setupConvergeOnProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", "review_clean_rounds=1")
	require.Equal(t, 0, code)
	return dir
}

// TestReviewConvergeOn_BlockingCleanAllUnclean records a round whose only
// surviving finding is medium: clean and converged under blocking, then
// switching to all re-evaluates the SAME recorded round as unclean in --status
// with no re-record (§3.2, §3.3, §3.4).
func TestReviewConvergeOn_BlockingCleanAllUnclean(t *testing.T) {
	dir := setupConvergeOnProject(t)

	out, stderr, code := recordRound(t, dir,
		`{"severity":"medium","category":"ambiguity","location":"L1","finding":"soft","suggestion":"clarify"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, true, out["clean"], "a medium-only survivor is clean under blocking")
	assert.Equal(t, true, out["converged"], "one clean round converges (review_clean_rounds=1)")

	// --status agrees under the default blocking policy.
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	status := parseStatusJSON(t, stdout)
	assert.Equal(t, true, status["converged"])
	rounds := status["review_rounds"].([]any)
	require.Len(t, rounds, 1)
	assert.Equal(t, true, rounds[0].(map[string]any)["clean"])

	// Switch to all — no re-record — and the same round re-evaluates unclean.
	_, _, code = runTP(t, dir, "set", "--workflow", "review_converge_on=all")
	require.Equal(t, 0, code)

	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	status = parseStatusJSON(t, stdout)
	assert.Equal(t, false, status["converged"], "under all a surviving medium unconverges the round")
	assert.Equal(t, float64(0), status["consecutive_clean"])
	rounds = status["review_rounds"].([]any)
	assert.Equal(t, false, rounds[0].(map[string]any)["clean"], "the recorded round re-evaluated live")

	// The findings file was not re-recorded: still one round.
	require.Len(t, rounds, 1)
}

// TestReviewConvergeOn_ResolveWontfixAfterRecordCleans records a round with a
// surviving high (blocking) finding, then resolves it wontfix; --status
// re-evaluates the round as clean without re-recording (§3.4, §3.5).
func TestReviewConvergeOn_ResolveWontfixAfterRecordCleans(t *testing.T) {
	dir := setupConvergeOnProject(t)

	out, stderr, code := recordRound(t, dir,
		`{"severity":"high","category":"completeness","location":"L1","finding":"missing","suggestion":"add"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, false, out["clean"], "a surviving high finding blocks under blocking")

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Equal(t, false, parseStatusJSON(t, stdout)["converged"])

	// Resolve the finding wontfix in the recorded round file.
	roundFile := filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson")
	_, _, code = runTP(t, dir, "review", "--resolve", roundFile, "0", "wontfix", "verifier: accepted risk")
	require.Equal(t, 0, code)

	// --status re-reads the round file: zero survivors -> clean -> converged.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	status := parseStatusJSON(t, stdout)
	assert.Equal(t, true, status["converged"], "resolving the blocking finding wontfix cleans the round")
	assert.Equal(t, float64(1), status["consecutive_clean"])
	rounds := status["review_rounds"].([]any)
	require.Len(t, rounds, 1, "no re-record: still one round")
	assert.Equal(t, true, rounds[0].(map[string]any)["clean"])
}

// TestReviewConvergeOn_InvalidStoredExitsOne: a consuming command that resolves
// an invalid review_converge_on winning from a stored layer exits 1 with the
// legal-values hint (§3.3). Write-time validation (exit 2) is separate.
func TestReviewConvergeOn_InvalidStoredExitsOne(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	// A task file with an invalid stored override (bypasses set's write-time
	// validation, mirroring an env / .tp/config.json / hand-edited value).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","workflow":{"review_converge_on":"bogus"},"tasks":[]}`), 0o600))

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	assert.Equal(t, 1, code, "--status exits 1 on an invalid stored review_converge_on")
	assert.Contains(t, stderr, "must be one of: blocking, all")

	_, stderr, code = recordRound(t, dir, "")
	assert.Equal(t, 1, code, "--record exits 1 on an invalid stored review_converge_on")
	assert.Contains(t, stderr, "must be one of: blocking, all")
}

// TestReviewConvergeOn_AuditUnaffected: audit convergence is status-based and
// never reads review_converge_on — an invalid stored value does not fault
// audit, and audit convergence is unchanged (§3.4, acceptance).
func TestReviewConvergeOn_AuditUnaffected(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	// Invalid review_converge_on stored — must not affect audit at all.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","workflow":{"review_converge_on":"bogus","audit_clean_rounds":1},"tasks":[]}`), 0o600))

	// A FAIL audit round is not clean (status-based, severity-blind).
	out, stderr, code := auditRecord(t, dir, `{"id":"x","status":"FAIL"}`+"\n")
	require.Equal(t, 0, code, "audit record ignores review_converge_on: %s", stderr)
	assert.Equal(t, false, out["converged"], "a FAIL round does not converge")

	// A PASS round converges (audit_clean_rounds=1) — review_converge_on
	// (even invalid) has no bearing on audit.
	out, _, code = auditRecord(t, dir, `{"id":"x","status":"PASS"}`+"\n")
	require.Equal(t, 0, code)
	assert.Equal(t, true, out["converged"], "a clean audit round converges regardless of review_converge_on")

	// audit --status resolves fine (exit 0) despite the invalid stored value.
	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "audit --status does not validate review_converge_on")
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, true, status["converged"])
}

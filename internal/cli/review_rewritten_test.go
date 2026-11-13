package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts a rewritten-spec fixture moves between: they share no sentence.
const (
	epochOldSpec = "# Spec\n\n## 1. Alpha\n\nRule one says the widget is blue.\n"
	epochNewSpec = "# Spec\n\n## 1. Gamma\n\nA wholly different document.\n"
)

// epochHighRow is a review finding about the old text, with a marker no other
// fixture text holds.
const epochHighRow = `{"evidence":"line 5","severity":"high","category":"clarity","location":"§1","finding":"MARKER-OLD-TEXT rule one is ambiguous","suggestion":"s"}` + "\n"

// rewrittenFixture records review round 1 with epochHighRow over the old text,
// then replaces the spec with the new one, and returns the directory.
func rewrittenFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	_, stderr, code := recordRound(t, dir, epochHighRow)
	require.Equal(t, 0, code, "record round 1: %s", stderr)
	writeSpec(t, dir, epochNewSpec)
	return dir
}

func writeSpec(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(body), 0o600))
}

func stateJSONBytes(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	return string(data)
}

func errorEnvelope(t *testing.T, stderr string) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env), "stderr is the error envelope: %s", stderr)
	return env
}

// TestReviewRewritten_EmptyEvidenceIsAUsageError: the evidence is required, and
// blank evidence is no evidence. Exit 2, naming the flag, and nothing written.
func TestReviewRewritten_EmptyEvidenceIsAUsageError(t *testing.T) {
	t.Parallel()
	for _, evidence := range []string{"", "   "} {
		dir := rewrittenFixture(t)
		before := stateJSONBytes(t, dir)
		_, stderr, code := runTP(t, dir, "review", "spec.md", "--rewritten", evidence)
		assert.Equal(t, 2, code, "evidence %q: %s", evidence, stderr)
		assert.Contains(t, errorEnvelope(t, stderr)["error"], "--rewritten requires evidence", "evidence %q", evidence)
		assert.Equal(t, before, stateJSONBytes(t, dir), "evidence %q: a refusal writes nothing", evidence)
	}
}

// TestReviewRewritten_ARoundInFlightIsRefused: a review round emitted and not
// recorded was built from the lists the epoch cuts, so the epoch is refused
// with exit 3 — the emission guard's code — and the hint names the round's
// record and, since the round was emitted over the old text, its discard as
// the operator's alternative.
func TestReviewRewritten_ARoundInFlightIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	_, stderr, code := recordRound(t, dir, epochHighRow)
	require.Equal(t, 0, code, "record round 1: %s", stderr)
	_, stderr, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emit round 2: %s", stderr)
	writeSpec(t, dir, epochNewSpec)
	before := stateJSONBytes(t, dir)

	_, stderr, code = runTP(t, dir, "review", "spec.md", "--rewritten", "replaced the spec")
	assert.Equal(t, 3, code, stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "review round 2 is in flight")
	assert.Contains(t, env["hint"], "tp review spec.md --record <file>")
	assert.Contains(t, env["hint"], "--force")
	assert.Equal(t, before, stateJSONBytes(t, dir), "a refusal writes nothing")
}

// TestReviewRewritten_AnAuditRoundInFlightIsRefused: the same holds for the
// audit phase, whose prior-round section and acceptance carry the epoch cuts
// too; the hint names tp audit's record.
func TestReviewRewritten_AnAuditRoundInFlightIsRefused(t *testing.T) {
	t.Parallel()
	dir := rewrittenFixture(t)
	markRoundEmitted(t, dir, "audit")

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--rewritten", "replaced the spec")
	assert.Equal(t, 3, code, stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "audit round 1 is in flight")
	assert.Contains(t, env["hint"], "tp audit spec.md --record <file>")
}

// TestReviewRewritten_NothingRewrittenIsRefused: a spec whose hash equals the
// last recorded round's snapshot hash was not rewritten, so there is no epoch
// to start: exit 2, and nothing written.
func TestReviewRewritten_NothingRewrittenIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	_, stderr, code := recordRound(t, dir, epochHighRow)
	require.Equal(t, 0, code, stderr)
	before := stateJSONBytes(t, dir)

	_, stderr, code = runTP(t, dir, "review", "spec.md", "--rewritten", "replaced the spec")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, errorEnvelope(t, stderr)["error"], "nothing was rewritten")
	assert.Equal(t, before, stateJSONBytes(t, dir))
}

// TestReviewRewritten_NoRecordedRoundIsRefused: with no recorded round there is
// no earlier text to cut, so the call is refused as a usage error and creates
// no state directory.
func TestReviewRewritten_NoRecordedRoundIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, epochNewSpec)
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--rewritten", "replaced the spec")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, errorEnvelope(t, stderr)["error"], "no recorded round")
	assert.NoDirExists(t, filepath.Join(dir, ".tp-review"))
}

// TestReviewRewritten_AppendsAnEpochAfterEachPhasesLastRound: the epoch is
// placed after the last recorded round of each phase, holds the evidence and
// a time, and both phases' --status report it.
func TestReviewRewritten_AppendsAnEpochAfterEachPhasesLastRound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	_, stderr, code := recordRound(t, dir, epochHighRow)
	require.Equal(t, 0, code, stderr)
	_, stderr, code = recordRound(t, dir, "")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = auditRecord(t, dir, `{"id":"a","status":"PASS"}`+"\n")
	require.Equal(t, 0, code, stderr)
	writeSpec(t, dir, epochNewSpec)

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--rewritten", "replaced the spec wholesale")
	require.Equal(t, 0, code, stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	epoch, ok := out["epoch"].(map[string]any)
	require.True(t, ok, "the payload names the epoch: %s", stdout)
	assert.Equal(t, float64(2), epoch["after_review_round"])
	assert.Equal(t, float64(1), epoch["after_audit_round"])
	assert.Equal(t, "replaced the spec wholesale", epoch["evidence"])
	assert.NotEmpty(t, epoch["at"])

	for _, phase := range []string{"review", "audit"} {
		stdout, stderr, code = runTP(t, dir, phase, "spec.md", "--status")
		require.Equal(t, 0, code, "%s --status: %s", phase, stderr)
		var status map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &status))
		epochs, ok := status["epochs"].([]any)
		require.True(t, ok, "%s --status reports the epochs: %s", phase, stdout)
		require.Len(t, epochs, 1, phase)
		assert.Equal(t, epoch, epochs[0], phase)
	}
}


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


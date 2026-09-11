package cli_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPayloadRepo writes spec.md and its adjacent task file (no git, so the
// working tree is empty of changes) and returns the dir.
func newPayloadRepo(t *testing.T, tasksJSON string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":`+tasksJSON+`}`), 0o600))
	return dir
}

// writeConvergedRounds writes a review state with `review`/`audit` clean rounds
// stamped with the current spec hash, so the corresponding sequence converges.
func writeConvergedRounds(t *testing.T, dir string, review, audit int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	require.NoError(t, err)
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	mk := func(n int) []map[string]any {
		rounds := make([]map[string]any, 0, n)
		for i := range n {
			rounds = append(rounds, map[string]any{"round": i + 1, "clean": true, "spec_hash": hash})
		}
		return rounds
	}
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	st := map[string]any{"spec": "spec.md", "review_rounds": mk(review), "audit_rounds": mk(audit)}
	out, err := json.Marshal(st)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), out, 0o600))
}

func nextAction(res map[string]any) (command any, payload map[string]any) {
	na := res["next_action"].(map[string]any)
	p, _ := na["payload"].(map[string]any)
	return na["command"], p
}

func TestResume_ImplementPayloadPrefersWIP(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)
	cmd, payload := nextAction(res)
	assert.Equal(t, "tp next", cmd)
	assert.Equal(t, "t1", payload["task"].(map[string]any)["id"])
	assert.Equal(t, true, payload["wip"])
}

func TestResume_ImplementPayloadFirstReady(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	_, payload := nextAction(resumeResult(t, dir))
	assert.Equal(t, "t1", payload["task"].(map[string]any)["id"])
	assert.Equal(t, false, payload["wip"])
}

func TestResume_ImplementPayloadNoReadyBlocker(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":["missing"],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)
	_, payload := nextAction(res)
	assert.Nil(t, payload["task"], "no ready task previews as null")
	b := blockerByCode(res, "no-ready-task")
	require.NotNil(t, b)
	assert.Equal(t, []any{"missing"}, b["data"].(map[string]any)["blocked_by"])
}

// TestResume_SpecStaleBlockerNamesTheClearingSequence: a spec edited after its
// last recorded review round raises spec-stale, and the blocker names the two
// steps that clear it for this spec — emit a review round over the new text,
// then record it — instead of a bare "reconcile it".
func TestResume_SpecStaleBlockerNamesTheClearingSequence(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, dir, 2, 0)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\nedited after review\n"), 0o600))

	res := resumeResult(t, dir)
	assert.Equal(t, "implement", res["phase"])
	b := blockerByCode(res, "spec-stale")
	require.NotNil(t, b, "the edited spec raises spec-stale")
	spec, ok := b["data"].(map[string]any)["spec"].(string)
	require.True(t, ok, "data.spec names the spec")
	assert.Equal(t, res["spec"], spec, "data.spec is the resolved spec path")
	msg, ok := b["message"].(string)
	require.True(t, ok, "the blocker carries a message")
	assert.Contains(t, msg, "tp review "+spec+",", "the message names the emission for this spec")
	assert.Contains(t, msg, "tp review "+spec+" --record <file>", "the message names the recording for this spec")
}

func TestResume_ReviewPayloadFirstRound(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	res := resumeResult(t, dir)
	assert.Equal(t, "review", res["phase"])
	cmd, payload := nextAction(res)
	assert.True(t, strings.HasPrefix(cmd.(string), "tp review "), "review command literal")
	assert.Equal(t, float64(1), payload["round"])
	assert.Equal(t, float64(0), payload["unresolved_findings"], "the first round has zero unresolved findings")
}

func TestResume_AuditPayloadFirstRound(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)
	assert.Equal(t, "audit", res["phase"])
	cmd, payload := nextAction(res)
	assert.True(t, strings.HasPrefix(cmd.(string), "tp audit "), "audit command literal")
	assert.Equal(t, float64(1), payload["round"])
	assert.Equal(t, float64(0), payload["unresolved_findings"])
}

// TestResume_DecomposePayloadNullCommand: decompose is agent work with no single
// tp command, so next_action.command stays null. Its payload carries nothing of
// its own — only the unit next_units[0] names, since v0.35.0's §4.1 makes
// next_action a rendering of that array rather than a second opinion.
func TestResume_DecomposePayloadNullCommand(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	writeConvergedRounds(t, dir, 2, 0)
	res := resumeResult(t, dir)
	assert.Equal(t, "decompose", res["phase"])
	cmd, payload := nextAction(res)
	assert.Nil(t, cmd)
	assert.Equal(t, map[string]any{"unit": map[string]any{"kind": "decompose", "id": "spec"}}, payload)
}

func TestResume_ReleasePayloadNullCommand(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, dir, 0, 2)
	res := resumeResult(t, dir)
	assert.Equal(t, "release", res["phase"])
	cmd, payload := nextAction(res)
	assert.Nil(t, cmd)
	assert.Empty(t, payload)
}

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
		for i := 0; i < n; i++ {
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
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)
	cmd, payload := nextAction(res)
	assert.Equal(t, "tp next", cmd)
	assert.Equal(t, "t1", payload["task"].(map[string]any)["id"])
	assert.Equal(t, true, payload["wip"])
}

func TestResume_ImplementPayloadFirstReady(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	_, payload := nextAction(resumeResult(t, dir))
	assert.Equal(t, "t1", payload["task"].(map[string]any)["id"])
	assert.Equal(t, false, payload["wip"])
}

func TestResume_ImplementPayloadNoReadyBlocker(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":["missing"],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)
	_, payload := nextAction(res)
	assert.Nil(t, payload["task"], "no ready task previews as null")
	b := blockerByCode(res, "no-ready-task")
	require.NotNil(t, b)
	assert.Equal(t, []any{"missing"}, b["data"].(map[string]any)["blocked_by"])
}

func TestResume_ReviewPayloadFirstRound(t *testing.T) {
	dir := newPayloadRepo(t, `[]`)
	res := resumeResult(t, dir)
	assert.Equal(t, "review", res["phase"])
	cmd, payload := nextAction(res)
	assert.True(t, strings.HasPrefix(cmd.(string), "tp review "), "review command literal")
	assert.Equal(t, float64(1), payload["round"])
	assert.Equal(t, float64(0), payload["unresolved_findings"], "the first round has zero unresolved findings")
}

func TestResume_AuditPayloadFirstRound(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)
	assert.Equal(t, "audit", res["phase"])
	cmd, payload := nextAction(res)
	assert.True(t, strings.HasPrefix(cmd.(string), "tp audit "), "audit command literal")
	assert.Equal(t, float64(1), payload["round"])
	assert.Equal(t, float64(0), payload["unresolved_findings"])
}

func TestResume_DecomposePayloadNullCommand(t *testing.T) {
	dir := newPayloadRepo(t, `[]`)
	writeConvergedRounds(t, dir, 2, 0)
	res := resumeResult(t, dir)
	assert.Equal(t, "decompose", res["phase"])
	cmd, payload := nextAction(res)
	assert.Nil(t, cmd)
	assert.Empty(t, payload)
}

func TestResume_ReleasePayloadNullCommand(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, dir, 0, 2)
	res := resumeResult(t, dir)
	assert.Equal(t, "release", res["phase"])
	cmd, payload := nextAction(res)
	assert.Nil(t, cmd)
	assert.Empty(t, payload)
}

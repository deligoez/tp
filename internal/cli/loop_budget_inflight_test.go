package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseStatusJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out
}

// TestReviewStatus_SurfacesBudgetAndInFlight covers §10.1 (max_rounds,
// rounds_remaining) and §10.2 (in_flight_round + atomic snapshot write) for
// review.
func TestReviewStatus_SurfacesBudgetAndInFlight(t *testing.T) {
	dir := setupBudgetProject(t, "review_max_rounds") // cap = 2

	// Fresh state: cap set, zero rounds recorded, no snapshot.
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	assert.Equal(t, float64(2), out["max_rounds"])
	assert.Equal(t, float64(2), out["rounds_remaining"])
	assert.Nil(t, out["in_flight_round"])
	assert.Equal(t, false, out["budget_exhausted"], "cap set but 0 rounds recorded → not exhausted")

	// Record one dirty round: rounds_remaining drops to 1.
	_, _, code = recordRound(t, dir, dirtyRow)
	require.Equal(t, 0, code)
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	out = parseStatusJSON(t, stdout)
	assert.Equal(t, float64(2), out["max_rounds"])
	assert.Equal(t, float64(1), out["rounds_remaining"])
	assert.Nil(t, out["in_flight_round"], "no snapshot without a round file")

	// Start round 2 (writes snapshot-round-2.md atomically) without recording.
	_, _, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	out = parseStatusJSON(t, stdout)
	assert.Equal(t, float64(1), out["rounds_remaining"], "the started round is not yet recorded")
	assert.Equal(t, float64(2), out["in_flight_round"], "snapshot-round-2.md exists without review-round-2.ndjson")

	// No .tmp leftover from the atomic snapshot write.
	entries, err := os.ReadDir(filepath.Join(dir, ".tp-review", "spec"))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotEqual(t, ".tmp", filepath.Ext(e.Name()), "no .tmp leftover: %s", e.Name())
	}
}

// TestReviewStatus_BudgetNullWhenUncapped: §10.1 — max_rounds and
// rounds_remaining are null when no cap is set.
func TestReviewStatus_BudgetNullWhenUncapped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	assert.Nil(t, out["max_rounds"], "uncapped → null")
	assert.Nil(t, out["rounds_remaining"], "uncapped → null")
	assert.Nil(t, out["in_flight_round"])
}

// TestAuditStatus_SurfacesBudgetAndInFlight mirrors the review test for audit.
func TestAuditStatus_SurfacesBudgetAndInFlight(t *testing.T) {
	dir := setupBudgetProject(t, "audit_max_rounds") // cap = 2
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	// Record one FAIL audit round.
	_, _, code := auditRecord(t, dir, `{"id":"x","status":"FAIL"}`+"\n")
	require.Equal(t, 0, code)

	// Audit prompt emission for round 2 writes snapshot-round-2.md (1 < cap 2).
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "audit round 2 emits: %s", stderr)

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	assert.Equal(t, float64(2), out["max_rounds"])
	assert.Equal(t, float64(1), out["rounds_remaining"], "one recorded audit round")
	assert.Equal(t, float64(2), out["in_flight_round"], "snapshot-round-2.md exists without audit-round-2.ndjson")
}

// TestAuditStatus_BudgetNullWhenUncapped: §10.1 mirror for audit.
func TestAuditStatus_BudgetNullWhenUncapped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	assert.Nil(t, out["max_rounds"], "uncapped → null")
	assert.Nil(t, out["rounds_remaining"], "uncapped → null")
	assert.Nil(t, out["in_flight_round"])
}

// TestStatus_BudgetFieldsSurviveCompact: §8.4 — max_rounds, rounds_remaining,
// and in_flight_round survive --compact (a driver branches on remaining budget).
func TestStatus_BudgetFieldsSurviveCompact(t *testing.T) {
	dir := setupBudgetProject(t, "review_max_rounds")
	_, _, code := recordRound(t, dir, dirtyRow)
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "review", "spec.md") // start round 2 (in-flight)
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	assert.Equal(t, float64(2), out["max_rounds"], "max_rounds survives --compact")
	assert.Equal(t, float64(1), out["rounds_remaining"], "rounds_remaining survives --compact")
	assert.Equal(t, float64(2), out["in_flight_round"], "in_flight_round survives --compact")
}

// TestResume_NextActionRecordRoundWhenInFlight: §10.2 — when a round's
// snapshot exists without its round file, tp resume points at recording it.
func TestResume_NextActionRecordRoundWhenInFlight(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\na\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[]}`), 0o600))

	// Start review round 1 (writes snapshot-round-1.md) but don't record.
	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code)
	res := parseStatusJSON(t, stdout)
	assert.Equal(t, "review", res["phase"])
	na := res["next_action"].(map[string]any)
	payload := na["payload"].(map[string]any)
	assert.Equal(t, "record-round", payload["action"])
	assert.Equal(t, float64(1), payload["round"])
}

// TestResume_NextActionReviewWhenNoInFlight: the default review next_action
// still applies when no round is in flight (regression guard).
func TestResume_NextActionReviewWhenNoInFlight(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\na\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[]}`), 0o600))

	stdout, _, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code)
	res := parseStatusJSON(t, stdout)
	assert.Equal(t, "review", res["phase"])
	na := res["next_action"].(map[string]any)
	payload := na["payload"].(map[string]any)
	assert.Nil(t, payload["action"], "no in-flight round → no record-round action")
	assert.Equal(t, float64(1), payload["round"], "default next round is 1")
}

// TestRegressionPrompt_NamesBaselinePath: §10.3 — the regression prompt names
// the snapshot it diffs against.
func TestRegressionPrompt_NamesBaselinePath(t *testing.T) {
	dir := t.TempDir()
	spec1 := "# Spec\n## 1. A\noriginal\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec1), 0o600))

	// Round 1: no regression (no baseline).
	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)

	// Edit the spec; round 2 appends a regression prompt that names the
	// round-1 snapshot as its baseline.
	spec2 := "# Spec\n## 1. A\nrewritten\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec2), 0o600))
	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)
	text, ok := byRole["regression"]
	require.True(t, ok, "regression prompt appended at round 2")
	assert.Contains(t, text, "snapshot-round-1.md", "regression prompt names its baseline snapshot path")
}

// TestRegressionPrompt_Round1NotEmitted: §10.3 — round 1 has no baseline
// (snapshot-round-0.md does not exist), so the regression role is not emitted
// with an empty diff.
func TestRegressionPrompt_Round1NotEmitted(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\na\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)
	assert.NotContains(t, byRole, "regression", "round 1 has no baseline → regression not emitted")
}

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneOpenTask is a task list holding a single ready task.
const oneOpenTask = `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`

// staleSpecRepo is an implement-phase project whose spec was edited after its
// two recorded clean review rounds: spec-stale stands.
func staleSpecRepo(t *testing.T) string {
	t.Helper()
	dir := newPayloadRepo(t, oneOpenTask)
	writeConvergedRounds(t, dir, 2, 0)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\nedited after review\n"), 0o600))
	return dir
}

// budgetExhaustedRepo is a review-phase project at a 2-round cap whose latest
// round holds an undispositioned high finding: review-budget-exhausted stands.
func budgetExhaustedRepo(t *testing.T) string {
	t.Helper()
	dir := setupBudgetProject(t, "review_max_rounds")
	for range 2 {
		_, stderr, code := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code, stderr)
	}
	return dir
}

// blockedRepo is an implement-phase project whose only open task depends on a
// task that does not exist: no-ready-task stands.
func blockedRepo(t *testing.T) string {
	t.Helper()
	return newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":["missing"],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
}

// firstEscalateMessage returns the message of the first escalate-class blocker
// of a resume result, failing the test when none stands.
func firstEscalateMessage(t *testing.T, res map[string]any) string {
	t.Helper()
	arr, _ := res["blockers"].([]any)
	for _, e := range arr {
		m := e.(map[string]any)
		if m["class"] == "escalate" {
			msg, ok := m["message"].(string)
			require.True(t, ok, "an escalate blocker carries a message")
			return msg
		}
	}
	require.FailNow(t, "no escalate blocker stands", "%v", res["blockers"])
	return ""
}

// stepPreviewKeys are the next_action payload keys that preview the phase's own
// next step: the task tp next would claim (task, wip) and the round tp review or
// tp audit would run or record (round, action).
var stepPreviewKeys = []string{"task", "wip", "round", "action"}

// assertNextActionDefersToTheBlocker is the rule: with an escalate blocker
// standing, next_action's summary is that blocker's message, and nothing in it
// points at a step — command and brief_command are null and the payload holds
// no step preview — since nothing runs until the operator answers.
func assertNextActionDefersToTheBlocker(t *testing.T, res map[string]any, code string) map[string]any {
	t.Helper()
	require.NotNil(t, blockerByCode(res, code), "%s stands", code)
	na := res["next_action"].(map[string]any)
	assert.Equal(t, firstEscalateMessage(t, res), na["summary"], "the summary is the escalate blocker's message")
	for _, key := range []string{"command", "brief_command"} {
		v, present := na[key]
		assert.True(t, present, "%s stays a key", key)
		assert.Nil(t, v, "%s: nothing runs until the operator answers", key)
	}
	payload, ok := na["payload"].(map[string]any)
	require.True(t, ok, "payload stays an object, never null")
	for _, key := range stepPreviewKeys {
		assert.NotContains(t, payload, key, "payload previews no step")
	}
	return payload
}

// TestResume_SpecStaleNextActionIsTheBlocker: a stale spec in implement stops
// next_action from offering the next task, its brief, or its preview.
func TestResume_SpecStaleNextActionIsTheBlocker(t *testing.T) {
	t.Parallel()
	res := resumeResult(t, staleSpecRepo(t))
	require.Equal(t, "implement", res["phase"])
	payload := assertNextActionDefersToTheBlocker(t, res, "spec-stale")
	assert.Nil(t, res["next_action"].(map[string]any)["brief_command"], "no tp next --brief")
	assert.NotContains(t, payload, "task", "no task preview")
}

// TestResume_BudgetExhaustedNextActionIsTheBlocker: a review stopped at its cap
// stops next_action from offering another review round or its brief, while the
// count of the latest round's open findings — what the blocker asks to
// disposition — stays.
func TestResume_BudgetExhaustedNextActionIsTheBlocker(t *testing.T) {
	t.Parallel()
	res := resumeResult(t, budgetExhaustedRepo(t))
	require.Equal(t, "review", res["phase"])
	payload := assertNextActionDefersToTheBlocker(t, res, "review-budget-exhausted")
	assert.Nil(t, res["next_action"].(map[string]any)["brief_command"], "no tp review --round brief")
	assert.NotContains(t, payload, "round", "no round-step preview")
	assert.Equal(t, float64(1), payload["unresolved_findings"], "the open findings the blocker names stay")
}

// TestResume_NoBlockerNextActionIsUnchanged is the control: with no blocker,
// next_action keeps the phase's own command, brief, summary and task preview.
func TestResume_NoBlockerNextActionIsUnchanged(t *testing.T) {
	t.Parallel()
	res := resumeResult(t, newPayloadRepo(t, oneOpenTask))
	require.Empty(t, res["blockers"])
	na := res["next_action"].(map[string]any)
	assert.Equal(t, "tp next", na["command"])
	assert.Equal(t, "tp next --brief", na["brief_command"])
	assert.Equal(t, "claim the next ready task t1", na["summary"])
	payload := na["payload"].(map[string]any)
	assert.Equal(t, map[string]any{"id": "t1"}, payload["task"])
	assert.Equal(t, false, payload["wip"])
}

// TestResume_EmptyNextUnitsSummaryNamesWhatThePhaseAwaits pins spec/0.35.0.md
// §4.1: next_units is [] outside release in two cases — a phase whose work is
// blocked, and a phase awaiting an operator decision — and in both
// "next_action.summary names what the phase is waiting for". Each state below
// is one of those; the summary must name the blocker the phase waits on.
func TestResume_EmptyNextUnitsSummaryNamesWhatThePhaseAwaits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		repo func(*testing.T) string
		code string
	}{
		{"blocked work", blockedRepo, "no-ready-task"},
		{"operator decision: stale spec", staleSpecRepo, "spec-stale"},
		{"operator decision: round cap", budgetExhaustedRepo, "review-budget-exhausted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := resumeResult(t, tc.repo(t))
			require.NotEqual(t, "release", res["phase"])
			require.Empty(t, nextUnitsOf(t, res), "the state §4.1 describes: no unit outside release")
			assertNextActionDefersToTheBlocker(t, res, tc.code)
		})
	}
}

package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkedClass is the recurring class the fixtures below register a check for.
const checkedClass = "vague-number"

// checkedClassFixture is a task-file project whose one registered check, of
// checkedClass, runs cmd — so whether the check ran, and what it returned, is
// the only thing that differs between the cases below.
func checkedClassFixture(t *testing.T, cmd string) string {
	t.Helper()
	return suppressionFixture(t, `[{"class":"`+checkedClass+`","cmd":"`+cmd+`"}]`)
}

// recordPayload records rows as one round and decodes the --record payload.
func recordPayload(t *testing.T, dir string, rows ...string) map[string]any {
	t.Helper()
	raw := recordSuppressionRound(t, dir, rows...)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	return payload
}

// convergeWithCheckedClass records a clean round holding five rows of
// checkedClass, then clean rounds until the loop converges, and returns the
// payload of the --record that converged it.
func convergeWithCheckedClass(t *testing.T, dir string) map[string]any {
	t.Helper()
	payload := recordPayload(t, dir, fiveRowsOfClass(checkedClass)...)
	for range 3 {
		if done, _ := payload["done"].(bool); done {
			break
		}
		payload = recordPayload(t, dir, "")
	}
	require.Equal(t, true, payload["converged"], "the fixture must converge, or next_action's forward step is not in play")
	return payload
}

// statusPayload runs `tp review spec.md --status` with extra flags and decodes it.
func statusPayload(t *testing.T, dir string, extra ...string) (payload map[string]any, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"review", "spec.md", "--status"}, extra...)...)
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "status must be JSON: %s / %s", stdout, stderr)
	return payload, code
}

func reviewNextActionOf(payload map[string]any) string {
	s, _ := payload["next_action"].(string)
	return s
}

// TestReviewRecord_ACheckThatCannotRunDoesNotMechanizeItsClass: --record withheld
// a recurring class by its registration alone, so a check exiting 2 — one that
// could not run and verified nothing — left next_action with nothing to say
// about it. The class is not mechanized by a check that did not run, and
// next_action names the check to fix rather than asking for one to be
// registered. The passing check is the control: it still withholds the class.
func TestReviewRecord_ACheckThatCannotRunDoesNotMechanizeItsClass(t *testing.T) {
	t.Parallel()

	t.Run("exit 2: not mechanized, next_action names the check to fix", func(t *testing.T) {
		t.Parallel()
		payload := recordPayload(t, checkedClassFixture(t, "exit 2"), fiveRowsOfClass(checkedClass)...)
		assert.NotContains(t, payload["mechanized_classes"], checkedClass,
			"a check that could not run mechanizes nothing")
		na := reviewNextActionOf(payload)
		assert.Contains(t, na, `"`+checkedClass+`"`, "next_action names the class")
		assert.Contains(t, na, "fix the registered", "and says to fix its check")
		assert.Contains(t, na, "exited 2", "naming the exit the check returned")
		assert.NotContains(t, na, "tp set --workflow", "a check already exists, so none is to be registered")
	})

	t.Run("exit 0: mechanized as before", func(t *testing.T) {
		t.Parallel()
		payload := recordPayload(t, checkedClassFixture(t, "exit 0"), fiveRowsOfClass(checkedClass)...)
		assert.Equal(t, []any{checkedClass}, payload["mechanized_classes"])
		assert.Equal(t, nextRoundAction, reviewNextActionOf(payload))
	})
}

// TestReviewNextAction_AFailingCheckWithholdsDecompose: a converged loop told
// the agent to decompose while `--status --check` exited 1 on a registered
// check. Every mode that knows the check result names the check instead; the
// modes are driven over one state each, for a check that found violations and
// one that could not run.
func TestReviewNextAction_AFailingCheckWithholdsDecompose(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"exit 1", "exit 2"} {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()
			dir := checkedClassFixture(t, cmd)
			recorded := convergeWithCheckedClass(t, dir)

			checked, code := statusPayload(t, dir, "--check")
			require.Equal(t, true, checked["converged"])
			require.Equal(t, 1, code, "the registered check fails, so the gate does not pass")

			for mode, na := range map[string]string{"--status --check": reviewNextActionOf(checked), "--record": reviewNextActionOf(recorded)} {
				assert.NotContains(t, na, "decompose", "%s: a failing check withholds the forward step", mode)
				assert.NotContains(t, na, "tp import", "%s: and every import step", mode)
				assert.Contains(t, na, `"`+checkedClass+`"`, "%s: it names the check to fix first", mode)
			}
		})
	}
}

// TestReviewStatus_PlainStatusDoesNotClaimDecomposeIsSafe: plain --status runs
// no check, so over a registered one it cannot know the gate passes, and it
// must not send the agent straight to decomposition. It names the mode that
// runs the checks first.
func TestReviewStatus_PlainStatusDoesNotClaimDecomposeIsSafe(t *testing.T) {
	t.Parallel()
	dir := checkedClassFixture(t, "exit 2")
	convergeWithCheckedClass(t, dir)

	payload, code := statusPayload(t, dir)
	require.Equal(t, 0, code)
	require.Equal(t, true, payload["converged"])
	na := reviewNextActionOf(payload)
	assert.False(t, strings.HasPrefix(na, "decompose"), "plain --status does not claim decomposition is safe: %s", na)
	gate := strings.Index(na, "tp review spec.md --status --check")
	require.GreaterOrEqual(t, gate, 0, "it names the mode that runs the checks: %s", na)
	if at := strings.Index(na, "decompose"); at >= 0 {
		assert.Less(t, gate, at, "the check gate comes before any decomposition: %s", na)
	}
}

// TestReviewNextAction_APassingCheckStillDecomposes is the control for the two
// tests above: with the registered check passing, the modes that ran it name
// the forward step exactly as a project with no check does.
func TestReviewNextAction_APassingCheckStillDecomposes(t *testing.T) {
	t.Parallel()
	const forward = "decompose the spec into tasks, then tp import spec.tasks.json"
	dir := checkedClassFixture(t, "exit 0")
	recorded := convergeWithCheckedClass(t, dir)
	assert.Equal(t, forward, reviewNextActionOf(recorded), "--record")

	checked, code := statusPayload(t, dir, "--check")
	require.Equal(t, 0, code, "a passing check over a converged loop is the ship signal")
	assert.Equal(t, forward, reviewNextActionOf(checked), "--status --check")
}

// TestReviewRecord_ReportsTheChecksItRan: when --record runs the registered
// checks, its payload carries what they did as mechanical_checks, in the
// --status --check shape, so a reader can see why the verdict moved. A record
// that ran none carries no such key.
func TestReviewRecord_ReportsTheChecksItRan(t *testing.T) {
	t.Parallel()

	t.Run("ran: each entry reports ran and passed", func(t *testing.T) {
		t.Parallel()
		payload := recordPayload(t, checkedClassFixture(t, "exit 2"), fiveRowsOfClass(checkedClass)...)
		checks, ok := payload["mechanical_checks"].([]any)
		require.True(t, ok, "--record ran the check, so it reports it: %v", payload)
		require.Len(t, checks, 1)
		entry, _ := checks[0].(map[string]any)
		assert.Equal(t, checkedClass, entry["class"])
		assert.Equal(t, false, entry["ran"], "exit 2 could not run")
		assert.Equal(t, false, entry["passed"])
		assert.Equal(t, float64(2), entry["exit_code"])
	})

	t.Run("ran none: no key", func(t *testing.T) {
		t.Parallel()
		// One clean round with no candidate: nothing the checks could change.
		payload := recordPayload(t, checkedClassFixture(t, "exit 0"), "")
		require.Equal(t, false, payload["done"])
		assert.NotContains(t, payload, "mechanical_checks")
	})
}

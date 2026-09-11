package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mechanicalChecksByClass projects an emission's mechanical_checks array to
// its entries keyed by class.
func mechanicalChecksByClass(t *testing.T, stdout string) map[string]map[string]any {
	t.Helper()
	var payload struct {
		MechanicalChecks []map[string]any `json:"mechanical_checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "the emission must be JSON: %s", stdout)
	byClass := make(map[string]map[string]any, len(payload.MechanicalChecks))
	for _, entry := range payload.MechanicalChecks {
		class, _ := entry["class"].(string)
		byClass[class] = entry
	}
	return byClass
}

// TestReviewEmission_ACheckThatCouldNotRunSuppressesNothing closes a hole in
// the reviewer exclusion list: a registered check that could not run still
// had its class stamped into every role prompt under "do NOT report findings
// of these classes", so the class was verified by nothing and reported by
// nobody.
//
// The exit-code contract is the one spec/backlog/next-action-and-check-tell-
// the-truth.md §4 states: 0 passed, 1 found violations, and anything else —
// 2 or higher, which takes in the shell's 126 (cannot execute) and 127
// (command not found), a start failure or a timeout — could not run. A check
// that could not run is reported `ran: false` and its class leaves the list;
// a check that ran keeps suppressing its class whichever verdict it reached,
// which is what the exit-0 and exit-1 entries in the same fixture pin, so the
// narrowing cannot pass by dropping the sentence wholesale.
//
// Both emission sites are driven: the panel appends the sentence inside
// buildReviewPrompts, before the checks have run, and the standalone
// regression perspective appends it after, so a fix at one site alone fails
// the other arm.
func TestReviewEmission_ACheckThatCouldNotRunSuppressesNothing(t *testing.T) {
	t.Parallel()

	// A command no shell can find: exit 127, the commonest cannot-run case.
	const missing = "tp-no-such-check-command-7f3a"
	const checks = `[` +
		`{"class":"exit-two-class","cmd":"exit 2"},` +
		`{"class":"clean-class","cmd":"exit 0"},` +
		`{"class":"missing-cmd-class","cmd":"` + missing + `"},` +
		`{"class":"found-class","cmd":"echo violation; exit 1"}]`
	want := []string{"clean-class", "found-class"}

	assertRan := func(t *testing.T, site, stdout string) {
		t.Helper()
		checks := mechanicalChecksByClass(t, stdout)
		require.Len(t, checks, 4, "%s: every valid entry is reported", site)
		for class, ran := range map[string]bool{
			"exit-two-class": false, "missing-cmd-class": false,
			"clean-class": true, "found-class": true,
		} {
			entry := checks[class]
			require.NotNil(t, entry, "%s: %s is reported", site, class)
			assert.Equal(t, ran, entry["ran"], "%s: %s ran=%v (exit_code %v)", site, class, ran, entry["exit_code"])
		}
		assert.Equal(t, true, checks["clean-class"]["passed"], "%s: a clean check passes", site)
		for _, class := range []string{"exit-two-class", "missing-cmd-class", "found-class"} {
			assert.Equal(t, false, checks[class]["passed"], "%s: %s does not pass", site, class)
		}
	}

	t.Run("panel", func(t *testing.T) {
		t.Parallel()
		dir := exclusionFixture(t, checks)
		stdout, stderr, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code, "panel review failed: %s", stderr)
		assertRan(t, "panel", stdout)
		for i, text := range promptTexts(t, stdout) {
			got, ok := exclusionClasses(t, text)
			require.True(t, ok, "panel prompt %d still carries the sentence for the checks that ran", i)
			assert.Equal(t, want, got,
				"panel prompt %d: a check that could not run must not suppress its class", i)
		}
	})

	t.Run("regression", func(t *testing.T) {
		t.Parallel()
		dir := exclusionFixture(t, checks)
		stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression",
			"--diff-from", "baseline.md", "--findings", "regression-findings.ndjson")
		require.Equal(t, 0, code, "standalone regression failed: %s", stderr)
		assertRan(t, "regression", stdout)
		texts := promptTexts(t, stdout)
		require.Len(t, texts, 1)
		got, ok := exclusionClasses(t, texts[0])
		require.True(t, ok, "the regression prompt still carries the sentence for the checks that ran")
		assert.Equal(t, want, got,
			"regression prompt: a check that could not run must not suppress its class")
	})
}

// TestReviewEmission_OnlyCheckCouldNotRunAppendsNoSentence is the emptying
// case: when the one registered check could not run, nothing is mechanically
// checked this round, so no prompt carries the sentence at all — not one
// ending in an empty list. The control is the same fixture with a clean
// check, which does carry it, so the absence is attributable to the exit code.
func TestReviewEmission_OnlyCheckCouldNotRunAppendsNoSentence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cmd      string
		suppress bool
	}{
		{cmd: "exit 2", suppress: false},
		{cmd: "exit 0", suppress: true},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()
			dir := exclusionFixture(t, `[{"class":"only-class","cmd":"`+tc.cmd+`"}]`)
			stdout, stderr, code := runTP(t, dir, "review", "spec.md")
			require.Equal(t, 0, code, "panel review failed: %s", stderr)
			assert.Equal(t, tc.suppress, mechanicalChecksByClass(t, stdout)["only-class"]["ran"])
			for i, text := range promptTexts(t, stdout) {
				got, ok := exclusionClasses(t, text)
				if tc.suppress {
					require.True(t, ok, "prompt %d: a check that ran suppresses its class", i)
					assert.Equal(t, []string{"only-class"}, got)
					continue
				}
				assert.False(t, ok, "prompt %d: a check that could not run leaves nothing to suppress, got %v", i, got)
			}
		})
	}
}

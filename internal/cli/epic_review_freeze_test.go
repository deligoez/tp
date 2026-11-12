package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frozenSlice is milestone A with round 1 recorded holding one open row on
// epic:d and round 2 emitted and not recorded: round 2 was built from round
// 1's dispositions.
func frozenSlice(t *testing.T) (dir string, round2 epicReviewPayload) {
	t.Helper()
	dir = epicGraphDir(t, epicChain)
	recordEpicRows(t, dir, inboxRow(""))
	return dir, runEpicReview(t, dir, "--milestone", "A")
}

// assertResolveFrozen runs a disposition over the frozen slice and expects
// exit 3, both ways forward in order, and round 1's file untouched.
func assertResolveFrozen(t *testing.T, dir string, args ...string) {
	t.Helper()
	path := filepath.Join(dir, ".tp", "epic-review")
	before := frozenRoundFiles(t, path)
	_, stderr, code := runTP(t, dir, append([]string{"epic", "review", "--milestone", "A"}, args...)...)
	require.Equal(t, 3, code, "stderr: %s", stderr)
	msg, hint := epicErrorOf(t, stderr)
	assert.Contains(t, msg, "round 2")
	inbox, force := strings.Index(hint, "tp epic review --milestone A --record-inbox"), strings.Index(hint, "tp epic review --milestone A --force")
	require.GreaterOrEqual(t, inbox, 0, hint)
	assert.Greater(t, force, inbox, hint)
	assert.Contains(t, hint, "operator's decision")
	assert.Equal(t, before, frozenRoundFiles(t, path), "a refused disposition writes nothing")
}

// roundFiles reads every recorded round file under the epic review state.
func frozenRoundFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "*", "round-*.ndjson"))
	require.NoError(t, err)
	out := map[string]string{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		out[p] = string(data)
	}
	return out
}

// TestEpicReviewFreeze_ResolvingWontfixWhileARoundIsOutIsRefused: round 2
// was built with the row open; accepting it now would change a round agents
// already hold.
func TestEpicReviewFreeze_ResolvingWontfixWhileARoundIsOutIsRefused(t *testing.T) {
	t.Parallel()
	dir, _ := frozenSlice(t)
	assertResolveFrozen(t, dir, "--resolve", "0", "wontfix", "accepted")
}

// TestEpicReviewFreeze_ResolvingFixedWhileARoundIsOutIsRefused: a fixed
// resolution would add regression to the round already out.
func TestEpicReviewFreeze_ResolvingFixedWhileARoundIsOutIsRefused(t *testing.T) {
	t.Parallel()
	dir, _ := frozenSlice(t)
	assertResolveFrozen(t, dir, "--resolve", "0", "fixed")
	assertResolveFrozen(t, dir, "--resolve-all", "fixed")
}

// TestEpicReviewFreeze_AWontfixCannotNarrowTheRoundOut: a wontfix that would
// only shrink one role's asked set, leaving the panel the same, can no longer
// be built, so no re-emission could miss it: the resolve itself is refused,
// and the stored emission stays the one the agents hold.
func TestEpicReviewFreeze_AWontfixCannotNarrowTheRoundOut(t *testing.T) {
	t.Parallel()
	dir, round2 := frozenSlice(t)
	require.Contains(t, round2.Units, "epic:d", "the open row re-asks d")
	assertResolveFrozen(t, dir, "--resolve", "0", "wontfix", "d's title is fine")
	again := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, round2.Units, again.Units, "the re-emission is identical")
}

// TestEpicReviewFreeze_RecordingFirstLetsTheDispositionThrough: once the
// round out is recorded, --resolve works on its rows as before.
func TestEpicReviewFreeze_RecordingFirstLetsTheDispositionThrough(t *testing.T) {
	t.Parallel()
	dir, round2 := frozenSlice(t)
	writeInbox(t, &round2, map[string]string{round2.Prompts[0].Role: inboxRow("")})
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "wontfix", "accepted")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"round": 2`)
}

// TestEpicReemit_APurposeChangeUnderAPlacerOnlyCorpusRefuses: no role reads
// the scope unit, so it is filtered out; the purpose still reaches the epic
// units the placer reads, and changing it refuses the re-emission.
func TestEpicReemit_APurposeChangeUnderAPlacerOnlyCorpusRefuses(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicRoles(t, dir, map[string]string{"placer": purposeRoles["placer"]})
	_, stderr, code := runTP(t, dir, "epic", "set", "gates.A.purpose=ship the chain")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	first := runEpicReview(t, dir, "--milestone", "A")
	require.NotContains(t, first.Units, "scope:A", "no role reads the scope unit")
	_, stderr, code = runTP(t, dir, "epic", "set", "gates.A.purpose=ship the chain twice")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	_, stderr, code = runTP(t, dir, "epic", "review", "--milestone", "A")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "purposes changed: A")
}

// TestEpicReemit_RoleFanOutOverOneRoundIsIdempotent: three --role calls over
// one unrecorded round, as sibling units fan out, compare the whole panel
// each time and all exit 0.
func TestEpicReemit_RoleFanOutOverOneRoundIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	for _, role := range []string{"dod-satisfiability", "why-auditor", "phase-economist"} {
		out := runEpicReview(t, dir, "--milestone", "A", "--role", role)
		assert.Equal(t, 1, out.Round, role)
		assert.Equal(t, []string{role}, promptRoles(&out))
	}
}

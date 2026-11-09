package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicReview_ASliceWithoutAGateSaysSo: a slice that asks no gate unit
// tells every role that its gate questions do not apply; a slice that asks
// one does not.
func TestEpicReview_ASliceWithoutAGateSaysSo(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainGatedOnD)
	without := runEpicReview(t, dir, "--epics", "b")
	require.NotEmpty(t, without.Prompts)
	for _, p := range without.Prompts {
		assert.Contains(t, p.Prompt, "No gate unit is asked", "%s", p.Role)
	}
	with := runEpicReview(t, dir, "--milestone", "A")
	for _, p := range with.Prompts {
		assert.NotContains(t, p.Prompt, "No gate unit is asked", "%s", p.Role)
	}
}

// TestEpicLink_AnEmptyWhyGetsAnEdgeHint: link's refusal of an empty why
// answers with what to give the edge, not the file-oriented hint.
func TestEpicLink_AnEmptyWhyGetsAnEdgeHint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	_, stderr, code := runTP(t, dir, "epic", "link", "a", "c", " ")
	require.Equal(t, 1, code)
	assert.Contains(t, stderr, "non-empty why")
}

// TestEpicMilestoneAdd_ReportsItsAction: milestone add reports action
// "added", as link and unlink report theirs.
func TestEpicMilestoneAdd_ReportsItsAction(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	stdout, stderr, code := runTP(t, dir, "epic", "milestone", "add", "X", "--new-track")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "added", epicActionOf(t, stdout))
}

// TestEpicReviewResolve_AUsageErrorShowsTheForm: a malformed --resolve
// answers with its own usage form rather than the generic help pointer.
func TestEpicReviewResolve_AUsageErrorShowsTheForm(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--resolve", "0")
	require.Equal(t, 2, code)
	assert.Contains(t, stderr, "0-based rows of the slice's last recorded round", "the hint names where indices come from")
	assert.NotContains(t, stderr, "see 'tp --help'")
}

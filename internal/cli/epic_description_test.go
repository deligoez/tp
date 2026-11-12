package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicDescriptionGraph is a two-epic graph: a depends on b, neither described.
const epicDescriptionGraph = `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A step", "milestone": "A", "depends_on": [{"id": "b", "why": "a reads what b writes"}], "acceptance": "a ships"},
    {"id": "b", "title": "B step", "milestone": "A", "root": true, "acceptance": "b ships"}
  ]
}
`

// TestEpicSet_DescriptionWritesTheTextAndBlankUnsetsIt (spec/backlog/
// epic-description.md §1): tp epic set <id> description=<text> writes the
// text as given, and an empty or whitespace-only value leaves the key out of
// the file, as an unset optional field is.
func TestEpicSet_DescriptionWritesTheTextAndBlankUnsetsIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)

	_, stderr, code := runTP(t, dir, "epic", "set", "a", "description=Purpose: prove the chain.")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, "Purpose: prove the chain.", appliedEpic(t, &g, "a").Description)

	for _, blank := range []string{"description=", "description=  \n\t "} {
		_, stderr, code = runTP(t, dir, "epic", "set", "a", blank)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.NotContains(t, string(readEpicGraphFile(t, dir)), `"description"`, "%q unsets the description", blank)
		_, stderr, code = runTP(t, dir, "epic", "set", "a", "description=back")
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}
}

// TestEpicReviewStatus_ADescriptionEditReAsksTheDescribedEpicAlone is the
// own-block rule at the loop: after a recorded round, describing b makes the
// slice stale on epic:b only; a, which depends on b, is not re-asked.
func TestEpicReviewStatus_ADescriptionEditReAsksTheDescribedEpicAlone(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)

	_, stderr, code := runTP(t, dir, "epic", "set", "b", "description=Purpose: feed a.")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	stale := decodeStaleUnits(t, dir, slice...)
	require.NotNil(t, stale.StaleUnits)
	assert.Equal(t, []string{"epic:b"}, *stale.StaleUnits)
}

// TestEpicApply_SetRowCarriesAMultiLineDescription: an apply set row's JSON
// string value carries multi-line markdown with no shell quoting, and the
// file keeps it byte for byte.
func TestEpicApply_SetRowCarriesAMultiLineDescription(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	text := "## Purpose\n\nProve the chain.\n\n- non-goal: speed\n- open: which runner?\n"
	value, err := json.Marshal(text)
	require.NoError(t, err)
	payload := applyPayload(t, dir, `{"op":"set","id":"b","field":"description","value":`+string(value)+`}`)

	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, text, appliedEpic(t, &g, "b").Description)
}

// TestEpicAdd_TakesADescriptionAndTheLintKnowsTheKey: tp epic add accepts the
// key under its strict decode, and tp epic lint reports no unknown key for
// it; before the field existed both refused it.
func TestEpicAdd_TakesADescriptionAndTheLintKnowsTheKey(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)

	_, stderr, code := runTP(t, dir, "epic", "add", `{"id":"c","title":"C step","milestone":"A","root":true,"description":"Why c exists."}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, "Why c exists.", appliedEpic(t, &g, "c").Description)

	stdout, stderr, code := runTP(t, dir, "epic", "lint")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, stdout, "unknown-key")
}

// TestEpicAdd_AWhitespaceDescriptionIsNotWritten: a whitespace-only
// description is unset, so the write leaves the key out.
func TestEpicAdd_AWhitespaceDescriptionIsNotWritten(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)

	_, stderr, code := runTP(t, dir, "epic", "add", `{"id":"c","title":"C step","milestone":"A","root":true,"description":" \n "}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.False(t, strings.Contains(string(readEpicGraphFile(t, dir)), `"description"`))
}

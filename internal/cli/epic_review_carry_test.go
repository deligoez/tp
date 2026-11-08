package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeEpicRound stands in for --record: it writes round's rows into the
// slice's state directory, which is what a recorded round leaves behind.
func writeEpicRound(t *testing.T, dir string, out *epicReviewPayload, rows string) {
	t.Helper()
	path := filepath.Join(dir, out.StateDir, "round-"+strconv.Itoa(out.Round)+".ndjson")
	require.NoError(t, os.WriteFile(path, []byte(rows), 0o600))
}

// editEpicGraph rewrites .tp/epics.json with old replaced by replacement.
func editEpicGraph(t *testing.T, dir, old, replacement string) {
	t.Helper()
	path := filepath.Join(dir, ".tp", "epics.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), old, "the edit's anchor is in the graph")
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o600))
}

// TestEpicReview_UnchangedUnitsAreCarried is §9 row 7: after a clean round 1,
// round 2 asks no unit and emits no prompts; editing the why of b → c changes
// b's block and, through its dependency, a's hash, so round 3 asks exactly a
// and b. A hash over the own block only would leave a carried.
func TestEpicReview_UnchangedUnitsAreCarried(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	r1 := runEpicReview(t, dir, "--milestone", "A")
	require.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d"}, r1.Units)
	writeEpicRound(t, dir, &r1, "")

	r2 := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, 2, r2.Round)
	assert.Empty(t, r2.Units, "round 2 asks no unit")
	assert.Empty(t, r2.Prompts, "round 2 emits no prompts")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d"}, r2.Carried)
	assert.Contains(t, r2.NextAction, "empty file")
	writeEpicRound(t, dir, &r2, "")

	editEpicGraph(t, dir, "b needs c", "b needs c's parser")
	r3 := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, 3, r3.Round)
	assert.Equal(t, []string{"epic:a", "epic:b"}, r3.Units)
	assert.Equal(t, []string{"c"}, r3.Context, "the carried neighbour c is context")
	assert.Equal(t, []string{"epic:c", "epic:d"}, r3.Carried)
	for _, p := range r3.Prompts {
		assert.NotContains(t, p.Prompt, "epic: d\n", "%s carries d by id only", p.Role)
	}
}

// TestEpicReview_AnOpenFindingReasksItsUnit is §9 row 18: unlike tp ground, a
// unit whose finding is still open is asked again although its hash is
// unchanged. A carry on the hash alone would ask nothing.
func TestEpicReview_AnOpenFindingReasksItsUnit(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	r1 := runEpicReview(t, dir, "--epic", "b")
	writeEpicRound(t, dir, &r1,
		`{"severity":"medium","finding":"the why of b -> c is generic","location":"epic:b","evidence":"b needs c"}`+"\n")

	r2 := runEpicReview(t, dir, "--epic", "b")
	assert.Equal(t, []string{"epic:b"}, r2.Units)
	assert.NotEmpty(t, r2.Prompts)
}

// TestEpicReview_RegressionFollowsAFixedResolution is §9 row 17: after a clean,
// unedited round 2 no regression prompt runs; once the round-2 row is resolved
// fixed, round 3 re-asks its unit (D2) and appends regression. Appending it on
// a changed slice alone would miss the second case.
func TestEpicReview_RegressionFollowsAFixedResolution(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	r1 := runEpicReview(t, dir, "--milestone", "A")
	writeEpicRound(t, dir, &r1, "")
	r2 := runEpicReview(t, dir, "--milestone", "A")
	row := `{"severity":"low","finding":"a's title is terse","location":"epic:a","evidence":"title: A","resolved":{"status":"%s","evidence":"decided"}}` + "\n"
	writeEpicRound(t, dir, &r2, strings.Replace(row, "%s", "wontfix", 1))

	r3 := runEpicReview(t, dir, "--milestone", "A")
	assert.NotContains(t, promptRoles(&r3), "regression", "a clean, unedited round 2 guards nothing")

	writeEpicRound(t, dir, &r2, strings.Replace(row, "%s", "fixed", 1))
	r3 = runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"epic:a"}, r3.Units, "a fixed finding's unit is asked again")
	roles := promptRoles(&r3)
	require.NotEmpty(t, roles)
	assert.Equal(t, "regression", roles[len(roles)-1], "regression is appended after a fixed resolution")
	assert.Contains(t, r3.Prompts[len(r3.Prompts)-1].Prompt, "a's title is terse", "the prompt names the fixed finding")
}

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// droppedScopeRowSlice is milestone A under purposeRoles with round 1's open
// high row on scope:A, after scope-judge's file is deleted: no role reads the
// scope unit the row holds open.
func droppedScopeRowSlice(t *testing.T) string {
	t.Helper()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, purposeRoles)
	recordEpicRows(t, dir, `{"severity":"high","finding":"the threshold needs a load test epic","location":"scope:A","evidence":"threshold: ten users","role":"scope-judge"}`+"\n")
	require.NoError(t, os.Remove(filepath.Join(dir, ".tp", "epic-reviewers", "scope-judge.json")))
	return dir
}

// TestEpicReview_AnOpenRowOnAUnitNoRoleReadsRefusesTheEmission: the open row
// would otherwise vanish with its unit and let the slice converge; the
// emission refuses at exit 4, naming the unit, the row and the missing kind,
// and writes no state.
func TestEpicReview_AnOpenRowOnAUnitNoRoleReadsRefusesTheEmission(t *testing.T) {
	t.Parallel()
	dir := droppedScopeRowSlice(t)
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "scope:A")
	assert.Contains(t, stderr, "the threshold needs a load test epic")
	assert.Contains(t, stderr, "scope")
	assert.Contains(t, stderr, "--resolve")
	matches, err := filepath.Glob(filepath.Join(dir, ".tp", "epic-review", "*", "snapshot-round-2.json"))
	require.NoError(t, err)
	assert.Empty(t, matches, "no round 2 state is written")
}

// TestEpicReview_ResolvingTheDroppedRowLetsTheRoundEmit: disposing of the
// row with --resolve clears the refusal.
func TestEpicReview_ResolvingTheDroppedRowLetsTheRoundEmit(t *testing.T) {
	t.Parallel()
	dir := droppedScopeRowSlice(t)
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "wontfix", "the scope role is retired")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.NotContains(t, out.Units, "scope:A")
}

// TestEpicReview_RestoringTheRoleReasksTheDroppedUnit: putting the scope role
// back lets the round emit and asks it the unit the open row holds.
func TestEpicReview_RestoringTheRoleReasksTheDroppedUnit(t *testing.T) {
	t.Parallel()
	dir := droppedScopeRowSlice(t)
	writeEpicRoles(t, dir, map[string]string{"scope-judge": purposeRoles["scope-judge"]})
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"scope:A"}, out.Units)
	assert.Equal(t, []string{"scope-judge"}, promptRoles(&out))
}

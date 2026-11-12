package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicChainWithE is epicChain plus e, an epic of A nothing depends on, so it
// can move out of A or be deleted without breaking an edge.
var epicChainWithE = strings.Replace(epicChain, `{"id": "d", "title": "D", "milestone": "A"}`,
	`{"id": "d", "title": "D", "milestone": "A"},
    {"id": "e", "title": "E", "milestone": "A", "depends_on": [{"id": "d", "why": "e needs d"}]}`, 1)

// vanishingRow is round 1's one finding, on epic:e.
const vanishingRow = `{"severity":"medium","finding":"e belongs to the later milestone","location":"epic:e","evidence":"milestone: A"}` + "\n"

// eMovedToL1 is epicChainWithE with e moved to L1, the other track.
var eMovedToL1 = strings.Replace(epicChainWithE, `"title": "E", "milestone": "A"`, `"title": "E", "milestone": "L1"`, 1)

// eDeleted is epicChainWithE without e.
var eDeleted = epicChain

// TestEpicReemit_MovingAnEpicOutAsTheFixPasses: round 1 found e misplaced;
// moving e out of A was the fix, and the row is resolved fixed, so the next
// emission is not refused for the unit that left the slice.
func TestEpicReemit_MovingAnEpicOutAsTheFixPasses(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithE)
	recordEpicRows(t, dir, vanishingRow)
	rewriteEpicGraph(t, dir, eMovedToL1)
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "fixed", "moved e to L1")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, 2, out.Round)
	assert.NotContains(t, out.Units, "epic:e")
}

// TestEpicReemit_AnOpenRowOnAMovedEpicRefuses: the same move with the row
// left open would take the finding with the unit; the emission refuses at
// exit 4, naming the row, and the hint names --resolve.
func TestEpicReemit_AnOpenRowOnAMovedEpicRefuses(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithE)
	recordEpicRows(t, dir, vanishingRow)
	rewriteEpicGraph(t, dir, eMovedToL1)

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	msg, hint := epicErrorOf(t, stderr)
	assert.Contains(t, msg, "epic:e, round 1 row 0 (medium): e belongs to the later milestone")
	assert.Contains(t, hint, "--resolve 0")
}

// TestEpicReemit_DeletingAnEpicWithAnOpenRowRefusesUntilResolved: deleting e
// with its row open refuses; resolving the row lets the round emit.
func TestEpicReemit_DeletingAnEpicWithAnOpenRowRefusesUntilResolved(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithE)
	recordEpicRows(t, dir, vanishingRow)
	rewriteEpicGraph(t, dir, eDeleted)

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "epic:e")

	_, stderr, code = runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "wontfix", "e was dropped from the plan")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, 2, out.Round)
}

// TestEpicReview_AFixedRowOnAUnitNoRoleReadsPasses: Part B's refusal for a
// unit no role reads follows the same distinction: a row resolved fixed on
// it does not hold the emission.
func TestEpicReview_AFixedRowOnAUnitNoRoleReadsPasses(t *testing.T) {
	t.Parallel()
	dir := droppedScopeRowSlice(t)
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "fixed", "the threshold names a load test epic now")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.NotContains(t, out.Units, "scope:A")
}

package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ancestryChange is one row of tp epic set's ancestry_changed.
type ancestryChange struct {
	Milestone string   `json:"milestone"`
	Lost      []string `json:"lost"`
	Gained    []string `json:"gained"`
}

// runEpicSetAncestry runs tp epic set with arg in dir and returns the
// response's ancestry_changed, failing when the key is absent.
func runEpicSetAncestry(t *testing.T, dir, arg string) []ancestryChange {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "set", arg)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		AncestryChanged *[]ancestryChange `json:"ancestry_changed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.NotNil(t, out.AncestryChanged, "stdout: %s", stdout)
	return *out.AncestryChanged
}

// ancestryGraph is the track [E2, F, G, H] in the default chain.
const ancestryGraph = `{
  "version": 1,
  "milestones": [["E2", "F", "G", "H"]],
  "epics": [{"id": "e", "title": "E", "milestone": "E2", "root": true}]
}
`

// TestEpicSet_OpensAfterReportsChangedAncestry is spec/1.6.0.md §1's pilot
// follow-up: writing {G: [E2]} makes G and H, which opens after G, lose F;
// writing {G: [E2], H: [F, G]} leaves H's ancestry whole, so only G is listed.
func TestEpicSet_OpensAfterReportsChangedAncestry(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ancestryGraph)
	assert.Equal(t, []ancestryChange{
		{Milestone: "G", Lost: []string{"F"}, Gained: []string{}},
		{Milestone: "H", Lost: []string{"F"}, Gained: []string{}},
	}, runEpicSetAncestry(t, dir, `opens_after={"G": ["E2"]}`))

	dir = epicGraphDir(t, ancestryGraph)
	assert.Equal(t, []ancestryChange{
		{Milestone: "G", Lost: []string{"F"}, Gained: []string{}},
	}, runEpicSetAncestry(t, dir, `opens_after={"G": ["E2"], "H": ["F", "G"]}`))

	assert.Equal(t, []ancestryChange{},
		runEpicSetAncestry(t, dir, `opens_after={"G": ["E2"], "H": ["F", "G"]}`), "an unchanged map lists none")
}

// TestEpicSet_MilestonesReportsChangedAncestry: reordering a track swaps
// its first two milestones' ancestry and leaves the third's, which opens
// after both either way, unlisted; rows follow the new track order.
func TestEpicSet_MilestonesReportsChangedAncestry(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A", "B", "C"]],
  "epics": [{"id": "a", "title": "A", "milestone": "A", "root": true}]
}
`)
	assert.Equal(t, []ancestryChange{
		{Milestone: "B", Lost: []string{"A"}, Gained: []string{}},
		{Milestone: "A", Lost: []string{}, Gained: []string{"B"}},
	}, runEpicSetAncestry(t, dir, `milestones=[["B", "A", "C"]]`))
}

// TestEpicSet_FirstMilestonesWriteReportsNoAncestry: milestones that did not
// exist before the write had no ancestry to change, so a first write lists
// none, and a later one lists only the milestones it already held.
func TestEpicSet_FirstMilestonesWriteReportsNoAncestry(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [], "epics": []}`)
	assert.Equal(t, []ancestryChange{}, runEpicSetAncestry(t, dir, `milestones=[["A", "B", "C"]]`))

	assert.Equal(t, []ancestryChange{
		{Milestone: "B", Lost: []string{}, Gained: []string{"X"}},
		{Milestone: "C", Lost: []string{}, Gained: []string{"X"}},
	}, runEpicSetAncestry(t, dir, `milestones=[["A", "X", "B", "C"]]`), "X is new: B and C gain it, X itself is not listed")
}

// epicAncestryOf runs tp epic with args in dir and returns the response's
// ancestry_changed, failing when the key is absent.
func epicAncestryOf(t *testing.T, dir string, args ...string) []ancestryChange {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		AncestryChanged *[]ancestryChange `json:"ancestry_changed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.NotNil(t, out.AncestryChanged, "stdout: %s", stdout)
	return *out.AncestryChanged
}

// TestEpicMilestoneAdd_ReportsChangedAncestry: inserting X after F re-routes
// G's default opens_after through X, so G and H, which opens after G, gain X.
func TestEpicMilestoneAdd_ReportsChangedAncestry(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, ancestryGraph)
	assert.Equal(t, []ancestryChange{
		{Milestone: "G", Lost: []string{}, Gained: []string{"X"}},
		{Milestone: "H", Lost: []string{}, Gained: []string{"X"}},
	}, epicAncestryOf(t, dir, "milestone", "add", "X", "--after", "F"))
}

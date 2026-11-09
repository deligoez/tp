package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicLink_ALaterMilestoneDependencyGetsItsOwnHint: an edge from an A
// epic to a B epic of track [A, B] is refused with a hint that says how to
// reorder, not the hint about a why and a known target.
func TestEpicLink_ALaterMilestoneDependencyGetsItsOwnHint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "B", "root": true}
  ]}`)
	_, stderr, code := runTP(t, dir, "epic", "link", "a", "b", "a extends b's model")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "later-milestone-dependency")
	assert.Contains(t, stderr, "move the edge's target to this or an earlier milestone of the track, or move this epic to a later milestone")
	assert.NotContains(t, stderr, "non-empty why")
}

// refusalHintGraph is track [A] with one root epic a.
const refusalHintGraph = `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true}
  ]}`

// TestEpicSet_AGatesKeyNamingNoMilestoneGetsItsOwnHint: a gates key is not an
// edge, so its refusal names the milestone to key the gate by.
func TestEpicSet_AGatesKeyNamingNoMilestoneGetsItsOwnHint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, refusalHintGraph)
	_, stderr, code := runTP(t, dir, "epic", "set", `gates={"Nope":{"members":[]}}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "key a gate by a milestone a track lists")
	assert.NotContains(t, stderr, "drop the edge")
}

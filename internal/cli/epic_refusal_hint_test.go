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

// TestEpicSet_ATagCheckProviderNamingNoEpicGetsItsOwnHint: provided_by is
// not an edge either, so its refusal says to point it at a known epic.
func TestEpicSet_ATagCheckProviderNamingNoEpicGetsItsOwnHint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, refusalHintGraph)
	_, stderr, code := runTP(t, dir, "epic", "set", `tag_checks={"ui":{"provided_by":"zz"}}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "point provided_by at an epic the graph holds")
	assert.NotContains(t, stderr, "drop the edge")
}

// TestEpicLink_ADependencyOpensLaterGetsItsOwnHint: on tracks [[A], [B]]
// with B opening after A, an edge from a, a member of A's gate, to b in B is
// refused with a hint naming both milestones and the three ways out.
func TestEpicLink_ADependencyOpensLaterGetsItsOwnHint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"], ["B"]], "opens_after": {"B": ["A"]}, "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "B", "root": true}
  ]}`)
	_, stderr, code := runTP(t, dir, "epic", "link", "a", "b", "a extends b's model")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "dependency-opens-later")
	assert.Contains(t, stderr, `move the epic of \"B\" the message names to a milestone that opens no later than \"A\", or drop the dependency, or make the member deferrable`)
}

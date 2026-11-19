package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


// TestEpicLayout_RefusesWhatTheEndpointRefuses: the command shares the
// endpoint's rules, one refusal class each: a view outside the closed set,
// a parameter the view does not take, the one it needs missing, a depth
// that is not a number and a packing that is neither rows nor tree are
// usage errors (2); a milestone or epic the graph does not hold is an
// absent object (4); a repository without a graph is a missing file (3).
// Each error is JSON with a hint.
func TestEpicLayout_RefusesWhatTheEndpointRefuses(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, layoutGraph)
	for _, c := range []struct {
		args []string
		code int
	}{
		{[]string{"--view", "all"}, 2},
		{[]string{"--view", "milestones", "--epic", "r1"}, 2},
		{[]string{"--view", "scope"}, 2},
		{[]string{"--view", "neighbourhood", "--epic", "r1", "--depth", "two"}, 2},
		{[]string{"--pack", "diagonal"}, 2},
		{[]string{"--view", "scope", "--milestone", "M9"}, 4},
		{[]string{"--view", "neighbourhood", "--epic", "nope"}, 4},
	} {
		stdout, stderr, code := runTP(t, dir, append([]string{"epic", "layout"}, c.args...)...)
		assert.Equal(t, c.code, code, "%v stdout: %s stderr: %s", c.args, stdout, stderr)
		var e struct {
			Error string `json:"error"`
			Hint  string `json:"hint"`
		}
		require.NoError(t, json.Unmarshal([]byte(stderr), &e), "%v stderr: %s", c.args, stderr)
		assert.NotEmpty(t, e.Hint, c.args)
	}
	_, stderr, code := runTP(t, t.TempDir(), "epic", "layout")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
}
// TestEpicLayout_EpicNodesCarryTheirTitleAndState: an epic node names its
// title and its derived status, the vocabulary tp epic export's status
// prints, so one call is enough to draw and colour the map; a milestone
// node of the overview carries neither.
func TestEpicLayout_EpicNodesCarryTheirTitleAndState(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, layoutGraph)
	type node struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		State string `json:"state"`
	}
	var plan struct {
		Nodes []node `json:"nodes"`
	}
	stdout, stderr, code := runTP(t, dir, "epic", "layout")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.NotEmpty(t, plan.Nodes)
	for _, n := range plan.Nodes {
		assert.Equal(t, strings.ToUpper(n.ID), n.Title, n.ID)
		assert.Equal(t, "planned", n.State, n.ID)
	}
	var overview struct {
		Nodes []node `json:"nodes"`
	}
	stdout, stderr, code = runTP(t, dir, "epic", "layout", "--view", "milestones")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &overview))
	for _, n := range overview.Nodes {
		assert.Empty(t, n.Title+n.State, n.ID)
	}
}

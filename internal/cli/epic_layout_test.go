package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

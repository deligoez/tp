package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrief_ATaskOverlappingAClaimIsNotHandedOut: tp brief hands out work
// under the touches rule tp run uses (spec/backlog/worktree-dispatch.md
// §2.3). While t1 is claimed, t2, whose touches overlap t1's, is refused by
// id with exit 4 naming t1, and a bare tp brief passes over it to t4.
func TestBrief_ATaskOverlappingAClaimIsNotHandedOut(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	addTaskWithEstimate(t, dir, "t4", 9)
	for id, touches := range map[string]string{"t1": `["go.mod"]`, "t2": `["go.mod","go.sum"]`, "t4": `["docs/"]`} {
		_, stderr, code := runTP(t, dir, "set", id, "touches="+touches)
		require.Equal(t, 0, code, stderr)
	}
	_, stderr, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A", "--session", "sessC")
	require.Equal(t, 0, code, stderr)

	stdout, _, code := runTP(t, dir, "brief", "t2")
	assert.Equal(t, 4, code, "a task overlapping a claim is refused")
	var refusal struct {
		Done    bool   `json:"done"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &refusal), stdout)
	assert.False(t, refusal.Done)
	assert.Contains(t, refusal.Message, "t1")

	stdout, stderr, code = runTP(t, dir, "brief")
	require.Equal(t, 0, code, stderr)
	var brief struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Equal(t, "t4", brief.Task.ID, "the bare pick passes over t2")

	_, stderr, code = runTP(t, dir, "brief", "t1")
	assert.Equal(t, 0, code, "the claimed task still reads its own brief: %s", stderr)
}

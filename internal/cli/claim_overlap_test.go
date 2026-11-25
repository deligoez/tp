package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overlapRepo is claimsRepo with t4 added and touches set: t2 overlaps t1,
// t4 overlaps neither. Nothing is claimed yet.
func overlapRepo(t *testing.T) string {
	t.Helper()
	dir := claimsRepo(t)
	addTaskWithEstimate(t, dir, "t4", 9)
	for id, touches := range map[string]string{"t1": `["go.mod"]`, "t2": `["go.mod","go.sum"]`, "t4": `["docs/"]`} {
		_, stderr, code := runTP(t, dir, "set", id, "touches="+touches)
		require.Equal(t, 0, code, stderr)
	}
	return dir
}

// holdT1 claims t1 for an agent spawn, the claim tp brief screens against.
func holdT1(t *testing.T, dir string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "claim", "t1", "--spawn", "toolu_A", "--session", "sessC")
	require.Equal(t, 0, code, stderr)
}

// TestNext_PassesOverATaskOverlappingAClaim: tp next claims under the touches
// rule tp brief uses. While t1 is held, t2 (first by priority) overlaps it,
// so tp next claims t4 and leaves t2 open.
func TestNext_PassesOverATaskOverlappingAClaim(t *testing.T) {
	t.Parallel()
	dir := overlapRepo(t)
	holdT1(t, dir)

	stdout, stderr, code := runTP(t, dir, "next")
	require.Equal(t, 0, code, stderr)
	var next struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &next), stdout)
	assert.Equal(t, "t4", next.Task.ID, "tp next passes over t2")
	assert.Equal(t, "open", taskState(t, dir, "t2")["status"])
}


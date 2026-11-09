package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicStatus_CompactDropsEachMilestoneCriticalPath: the whole graph's
// critical_path carries the chain once, so --compact leaves each
// milestone's own out and keeps the top-level one.
func TestEpicStatus_CompactDropsEachMilestoneCriticalPath(t *testing.T) {
	t.Parallel()
	dir := downstreamDir(t, exportF9Epics(), nil)
	stdout, stderr, code := runTP(t, dir, "--compact", "epic", "status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		CriticalPath *json.RawMessage             `json:"critical_path"`
		Milestones   []map[string]json.RawMessage `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.NotNil(t, out.CriticalPath, "the top-level critical_path stays")
	require.NotEmpty(t, out.Milestones)
	for _, m := range out.Milestones {
		_, has := m["critical_path"]
		assert.False(t, has, "milestone %s carries critical_path", m["milestone"])
	}
}

// TestEpicReady_CompactDropsDependentsByMilestone: choosing what to start
// needs the ranking counts, not their split per milestone, so --compact
// leaves dependents_by_milestone out of every ready item and keeps the rest.
func TestEpicReady_CompactDropsDependentsByMilestone(t *testing.T) {
	t.Parallel()
	dir := downstreamDir(t, []downstreamEpic{dsEpic("a", "M1"), dsEpic("b", "M1", "a"), dsEpic("c", "M2", "b")}, nil)
	for _, args := range [][]string{{"--compact", "epic", "ready"}, {"--compact", "epic", "status"}} {
		stdout, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.NotContains(t, stdout, "dependents_by_milestone", "%v", args)
		assert.Contains(t, stdout, `"longest_chain": 2`, "%v", args)
	}
	stdout, _, _ := runTP(t, dir, "epic", "ready")
	assert.Contains(t, stdout, "dependents_by_milestone", "without --compact the split stays")
}

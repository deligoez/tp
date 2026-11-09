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
		CriticalPath *json.RawMessage            `json:"critical_path"`
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

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDashboardLive_DerivesTheThreePayloadsInOrder runs Live in-process, so
// the race detector sees its three derivations run side by side: the graph's
// export, the task file list and the summary, in that order and none failed.
func TestDashboardLive_DerivesTheThreePayloadsInOrder(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "epics.json"), []byte(`{"version": 1, "milestones": [["A"]], "epics": [
  {"id": "a", "title": "A", "milestone": "A", "root": true}
]}`), 0o600))
	t.Chdir(root)

	got := dashboardAPI{}.Live()
	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, p.Event)
		assert.Nil(t, p.Err, "%s failed: %+v", p.Event, p.Err)
	}
	assert.Equal(t, []string{"epic-export", "export-list", "summary"}, names)
}

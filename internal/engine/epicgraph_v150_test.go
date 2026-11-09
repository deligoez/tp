package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// epicGraphV150Fields carries every key v1.5.0 adds to the graph, in the
// canonical form WriteEpicGraph emits: a gate's threshold and met record, and
// an epic's condition, estimate, claim and done record.
const epicGraphV150Fields = `{
  "version": 1,
  "milestones": [
    [
      "A"
    ]
  ],
  "gates": {
    "A": {
      "members": [
        "a"
      ],
      "threshold": "ten paying users",
      "met": {
        "at": "2026-11-09T10:00:00Z",
        "evidence": "https://example.com/dashboard",
        "threshold": "ten paying users"
      }
    }
  },
  "epics": [
    {
      "id": "a",
      "title": "A",
      "milestone": "A",
      "estimate_minutes": 90,
      "claimed": {
        "at": "2026-11-08T09:00:00Z",
        "by": "agent-1"
      },
      "done": {
        "at": "2026-11-09T09:30:00Z",
        "evidence": "shipped in v2"
      }
    },
    {
      "id": "launch window",
      "title": "Launch window opens",
      "milestone": "A",
      "acceptance": "the store approves the listing",
      "root": true,
      "condition": true
    }
  ]
}
`

// rewriteEpicGraph writes g back over tpDir's graph file and returns the bytes written.
func rewriteEpicGraph(t *testing.T, tpDir string, g *model.EpicGraph) string {
	t.Helper()
	require.NoError(t, os.Remove(filepath.Join(tpDir, EpicGraphFile)))
	require.NoError(t, WriteEpicGraph(tpDir, g))
	got, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
	require.NoError(t, err)
	return string(got)
}

// TestEpicGraph_V150KeysLoadAndWriteBackUnchanged: §8's new keys are schema,
// not unknown keys, and a graph carrying all of them writes back byte-identically.
func TestEpicGraph_V150KeysLoadAndWriteBackUnchanged(t *testing.T) {
	tpDir := epicGraphDir(t, epicGraphV150Fields)

	g, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Empty(t, unknown)
	gate := g.Gates["A"]
	assert.Equal(t, "ten paying users", gate.Threshold)
	require.NotNil(t, gate.Met)
	assert.Equal(t, "ten paying users", gate.Met.Threshold)
	assert.Equal(t, 90, g.Epics[0].EstimateMinutes)
	require.NotNil(t, g.Epics[0].Claimed)
	assert.Equal(t, "agent-1", g.Epics[0].Claimed.By)
	require.NotNil(t, g.Epics[0].Done)
	assert.Equal(t, "shipped in v2", g.Epics[0].Done.Evidence)
	assert.True(t, g.Epics[1].Condition)

	assert.Equal(t, epicGraphV150Fields, rewriteEpicGraph(t, tpDir, g))
}


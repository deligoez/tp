package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGraphAnyOf is a graph in WriteEpicGraph's canonical form whose a
// depends on a plain edge and on an any-of group (spec/1.7.0.md §7).
const epicGraphAnyOf = `{
  "version": 1,
  "milestones": [
    [
      "A"
    ]
  ],
  "epics": [
    {
      "id": "a",
      "title": "A",
      "milestone": "A",
      "depends_on": [
        {
          "id": "b",
          "why": "a reads b"
        },
        {
          "any_of": [
            "c",
            "b2"
          ],
          "why": "either decision opens the API"
        }
      ]
    },
    {
      "id": "b",
      "title": "B",
      "milestone": "A"
    },
    {
      "id": "b2",
      "title": "B2",
      "milestone": "A"
    },
    {
      "id": "c",
      "title": "C",
      "milestone": "A"
    }
  ]
}
`

// TestEpicGraph_AnyOfIsAKnownDependencyKeyAndRoundTrips: any_of is schema,
// so it is no unknown-key, and a group is written back as
// {"any_of": [...], "why": ...} with its members in stored order and no id.
func TestEpicGraph_AnyOfIsAKnownDependencyKeyAndRoundTrips(t *testing.T) {
	tpDir := epicGraphDir(t, epicGraphAnyOf)
	g, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Empty(t, unknown)

	require.NoError(t, os.Remove(filepath.Join(tpDir, EpicGraphFile)))
	require.NoError(t, WriteEpicGraph(tpDir, g))
	got, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
	require.NoError(t, err)
	assert.Equal(t, epicGraphAnyOf, string(got))
}

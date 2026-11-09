package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGraphOpensAfter carries spec/1.6.0.md §1's opens_after in the canonical
// form WriteEpicGraph emits, an always-open [] entry included.
const epicGraphOpensAfter = `{
  "version": 1,
  "milestones": [
    [
      "A",
      "B"
    ],
    [
      "L"
    ]
  ],
  "opens_after": {
    "B": [],
    "L": [
      "A"
    ]
  },
  "epics": [
    {
      "id": "a",
      "title": "A",
      "milestone": "A",
      "root": true
    }
  ]
}
`

// TestEpicGraph_OpensAfterLoadsAndWritesBackUnchanged: opens_after is schema,
// not an unknown key, and a graph carrying it writes back byte-identically;
// a graph without it gains no key.
func TestEpicGraph_OpensAfterLoadsAndWritesBackUnchanged(t *testing.T) {
	tpDir := epicGraphDir(t, epicGraphOpensAfter)
	g, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Empty(t, unknown)
	assert.Equal(t, map[string][]string{"B": {}, "L": {"A"}}, g.OpensAfter)
	assert.Equal(t, epicGraphOpensAfter, rewriteEpicGraph(t, tpDir, g))

	g.OpensAfter = nil
	assert.NotContains(t, rewriteEpicGraph(t, tpDir, g), `"opens_after"`)
}

package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGraphEveryField holds every §2 field in the canonical form WriteEpicGraph
// emits: 2-space indentation, struct key order, sorted map keys, and content
// (non-ASCII, spaces, "<" and "&") kept as it is rather than escaped.
const epicGraphEveryField = `{
  "version": 1,
  "milestones": [
    [
      "A",
      "B",
      "C"
    ],
    [
      "L1",
      "L2"
    ]
  ],
  "gates": {
    "A": {
      "members": [
        "money-type",
        "rounding"
      ],
      "deferrable": [
        "ledger-export"
      ]
    }
  },
  "tag_checks": {
    "ui": {
      "provided_by": "ui-rule-checker"
    }
  },
  "epics": [
    {
      "id": "money-type",
      "title": "Money type",
      "milestone": "A",
      "tags": [
        "det"
      ],
      "depends_on": [
        {
          "id": "event store ç",
          "why": "amounts are event payload <& more>"
        }
      ],
      "acceptance": "Para birimi tutarları kuruş cinsinden…"
    },
    {
      "id": "event store ç",
      "title": "Event store",
      "milestone": "B"
    }
  ]
}
`

func epicGraphDir(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	tpDir := filepath.Join(root, ".tp")
	require.NoError(t, os.Mkdir(tpDir, 0o755))
	if content != "" {
		require.NoError(t, os.WriteFile(filepath.Join(tpDir, EpicGraphFile), []byte(content), 0o600))
	}
	return tpDir
}

func TestEpicGraph_RoundTripIsByteIdentical(t *testing.T) {
	tpDir := epicGraphDir(t, epicGraphEveryField)

	g, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Empty(t, unknown)
	require.Len(t, g.Epics, 2)
	assert.Equal(t, "event store ç", g.Epics[0].DependsOn[0].ID)
	assert.Equal(t, "ui-rule-checker", g.TagChecks["ui"].ProvidedBy)
	assert.Equal(t, []string{"ledger-export"}, g.Gates["A"].Deferrable)

	require.NoError(t, os.Remove(filepath.Join(tpDir, EpicGraphFile)))
	require.NoError(t, WriteEpicGraph(tpDir, g))
	got, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
	require.NoError(t, err)
	assert.Equal(t, epicGraphEveryField, string(got))
}

func TestEpicGraph_UnknownKeysCarryTheirPaths(t *testing.T) {
	tpDir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "owner": "x",
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"},
    {"id": "b", "title": "B", "milestone": "A", "trigger": "x", "checks": []}
  ]
}`)

	g, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	require.Len(t, g.Epics, 2)
	assert.Equal(t, []EpicGraphUnknownKey{
		{Path: "owner", Key: "owner"},
		{Path: "epics[1].checks", Key: "checks", EpicID: "b"},
		{Path: "epics[1].trigger", Key: "trigger", EpicID: "b"},
	}, unknown)
}


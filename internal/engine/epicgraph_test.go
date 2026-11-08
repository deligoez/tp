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

// TestEpicGraph_NestedUnknownKeysCarryTheirPaths: §2 bounds every object of the
// schema, so a key inside a gate, a tag check or a dependency edge is as
// unknown as one at the top level; a check of the top level and the epics
// alone would return none of these.
func TestEpicGraph_NestedUnknownKeysCarryTheirPaths(t *testing.T) {
	tpDir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "gates": {"A": {"members": ["a"], "extra": 1}},
  "tag_checks": {"ui": {"provided_by": "a", "command": "x"}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "a", "why": "b reads a", "weight": 2}]}
  ]
}`)

	_, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Equal(t, []EpicGraphUnknownKey{
		{Path: "gates.A.extra", Key: "extra"},
		{Path: "tag_checks.ui.command", Key: "command"},
		{Path: "epics[1].depends_on[0].weight", Key: "weight", EpicID: "b"},
	}, unknown)
}

func TestEpicGraph_MissingFileIsNotAParseError(t *testing.T) {
	_, _, err := LoadEpicGraph(epicGraphDir(t, ""))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEpicGraphNotFound)
	var malformed *MalformedConfigError
	assert.False(t, errors.As(err, &malformed), "a missing file is not a parse error")

	_, _, err = LoadEpicGraph(epicGraphDir(t, `{"epics": [`))
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrEpicGraphNotFound)
	assert.True(t, errors.As(err, &malformed), "a parse error is MalformedConfigError")
}

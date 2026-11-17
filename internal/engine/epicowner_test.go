package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGraphWithOwners declares two owners and names them on a work epic and a
// condition, the second epic's list out of declaration order: the order an
// owner list is written in is kept, never sorted.
const epicGraphWithOwners = `{
  "version": 1,
  "milestones": [
    [
      "A"
    ]
  ],
  "owners": [
    {
      "handle": "ana",
      "name": "Ana A."
    },
    {
      "handle": "ben"
    }
  ],
  "epics": [
    {
      "id": "w",
      "title": "Work",
      "milestone": "A",
      "root": true,
      "owner": [
        "ben",
        "ana"
      ]
    },
    {
      "id": "c",
      "title": "Decide",
      "milestone": "A",
      "root": true,
      "condition": true,
      "owner": [
        "ana"
      ]
    }
  ]
}
`

func TestEpicOwner_TheSetAndTheFieldRoundTripByteIdentical(t *testing.T) {
	tpDir := epicGraphDir(t, epicGraphWithOwners)

	g, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Empty(t, unknown)

	require.NoError(t, os.Remove(filepath.Join(tpDir, EpicGraphFile)))
	require.NoError(t, WriteEpicGraph(tpDir, g))
	got, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
	require.NoError(t, err)
	assert.Equal(t, epicGraphWithOwners, string(got))
}

// TestEpicOwner_AKeyInsideAnOwnerEntryOutsideTheSchemaIsUnknown: an owner
// entry holds a handle and a name and nothing else tp would read, an email
// included.
func TestEpicOwner_AKeyInsideAnOwnerEntryOutsideTheSchemaIsUnknown(t *testing.T) {
	tpDir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]],
  "owners": [{"handle": "ana"}, {"handle": "ben", "email": "x"}],
  "epics": [{"id": "w", "title": "W", "milestone": "A", "root": true}]}`)

	_, unknown, err := LoadEpicGraph(tpDir)
	require.NoError(t, err)
	assert.Equal(t, []EpicGraphUnknownKey{{Path: "owners[1].email", Key: "email"}}, unknown)
}

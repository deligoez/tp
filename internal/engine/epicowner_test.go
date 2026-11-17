package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
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

// ownedGraph is a one-milestone graph declaring handles, with one root work
// epic per owner list given.
func ownedGraph(handles []string, owners ...[]string) *model.EpicGraph {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}}
	for _, h := range handles {
		g.Owners = append(g.Owners, model.EpicOwner{Handle: h})
	}
	for i, o := range owners {
		g.Epics = append(g.Epics, model.Epic{ID: fmt.Sprintf("e%d", i), Title: "E", Milestone: "A", Root: true, Owner: o})
	}
	return g
}

// ownerFindings keeps the findings of the owner rules.
func ownerFindings(g *model.EpicGraph) []EpicLintFinding {
	var out []EpicLintFinding
	for _, f := range LintEpicGraph(g, nil) {
		if strings.Contains(f.Rule, "owner") {
			out = append(out, f)
		}
	}
	return out
}

func TestEpicOwner_AHandleTheSetDoesNotDeclareIsUnknownOwner(t *testing.T) {
	got := ownerFindings(ownedGraph([]string{"ana"}, []string{"ana"}, []string{"ana", "zed"}))

	require.Len(t, got, 1)
	assert.Equal(t, EpicRuleUnknownOwner, got[0].Rule)
	assert.Equal(t, EpicSeverityError, got[0].Severity)
	assert.Equal(t, []string{"e1"}, got[0].Epics)
	assert.Contains(t, got[0].Message, `"zed"`)
}

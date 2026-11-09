package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// opensAfterGraph is tracks with one root epic per milestone, named after
// the milestone in lower case, and opens_after as given.
func opensAfterGraph(tracks [][]string, opensAfter map[string][]string) *model.EpicGraph {
	g := &model.EpicGraph{Version: 1, Milestones: tracks, OpensAfter: opensAfter}
	for _, track := range tracks {
		for _, m := range track {
			g.Epics = append(g.Epics, model.Epic{ID: strings.ToLower(m), Title: m, Milestone: m, Root: true})
		}
	}
	return g
}

// markDone puts a done record on epic id of g.
func markDone(g *model.EpicGraph, id string) {
	g.Epics[epicIndex(g, id)].Done = epicDoneRecord()
}

// openOf lists, in track order, the milestones of g that are open.
func openOf(t *testing.T, g *model.EpicGraph) []string {
	t.Helper()
	gates := gatesOf(t, g)
	out := []string{}
	for _, track := range g.Milestones {
		for _, m := range track {
			if gates[m].Open {
				out = append(out, m)
			}
		}
	}
	return out
}

// TestMilestoneGates_OpensAfterSkipsAnEarlierMilestone is C2.1: on track
// [P, Q, R] with R opening after P, a green P opens Q and R though Q is red;
// a red P leaves only P open.
func TestMilestoneGates_OpensAfterSkipsAnEarlierMilestone(t *testing.T) {
	g := opensAfterGraph([][]string{{"P", "Q", "R"}}, map[string][]string{"R": {"P"}})
	assert.Equal(t, []string{"P"}, openOf(t, g), "a red P holds Q and R")
	markDone(g, "p")
	assert.Equal(t, []string{"P", "Q", "R"}, openOf(t, g), "R opens after P, not after the red Q")
}


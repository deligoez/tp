package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapNode is one synthetic node: its id, group, layer and forest column.
type mapNode struct {
	id    string
	group string
	layer int
	col   float64
}

func mapInput(width float64, bands []LayoutBand, nodes ...mapNode) MapRowsInput {
	in := MapRowsInput{Layer: map[string]int{}, Col: map[string]float64{}, Group: map[string]string{}, Bands: bands, Width: width}
	for _, n := range nodes {
		in.IDs = append(in.IDs, n.id)
		in.Layer[n.id], in.Col[n.id], in.Group[n.id] = n.layer, n.col, n.group
	}
	return in
}

// threeBands is a synthetic plan: bands A (layers 0-2) and B (layers 1-3)
// share layers and make one row; C (layer 4) is a row alone. B's epics sit
// left of A's in the forest.
func threeBands() MapRowsInput {
	return mapInput(20, []LayoutBand{{Name: "A", From: 0, To: 2}, {Name: "B", From: 1, To: 3}, {Name: "C", From: 4, To: 4}},
		mapNode{"a0", "A", 0, 6}, mapNode{"a1", "A", 1, 6}, mapNode{"a2", "A", 1, 7}, mapNode{"a3", "A", 2, 6},
		mapNode{"b0", "B", 1, 0}, mapNode{"b1", "B", 2, 0}, mapNode{"b2", "B", 3, 1},
		mapNode{"c0", "C", 4, 3})
}

func mapBlockOf(t *testing.T, blocks []MapBlock, name string) MapBlock {
	t.Helper()
	for _, b := range blocks {
		if b.Milestone == name {
			return b
		}
	}
	require.Failf(t, "no block", "%s", name)
	return MapBlock{}
}

// TestMapRows_BandsSharingALayerMakeOneRow: A and B share layers 1-2 and
// make row 0 with two milestones; C, alone on layer 4, is row 1 and still
// gets a block of its own.
func TestMapRows_BandsSharingALayerMakeOneRow(t *testing.T) {
	t.Parallel()
	in := threeBands()
	got := MapRows(&in)
	require.Len(t, got.Rows, 2)
	assert.Equal(t, [2]int{0, 3}, [2]int{got.Rows[0].FromLayer, got.Rows[0].ToLayer})
	assert.Equal(t, 2, got.Rows[0].Milestones)
	assert.Equal(t, 7, got.Rows[0].Count)
	assert.Equal(t, 1, got.Rows[1].Milestones)
	assert.Equal(t, 1, mapBlockOf(t, got.Blocks, "C").Row)
	assert.Equal(t, 1, mapBlockOf(t, got.Blocks, "C").Count)
}


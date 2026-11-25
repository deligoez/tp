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
	assert.InDelta(t, got.Rows[1].FromY, mapBlockOf(t, got.Blocks, "C").LineY, 0, "a block names its line's header y")
	assert.InDelta(t, got.Rows[1].FromY+1, mapBlockOf(t, got.Blocks, "C").FromY, 0)
}

// TestMapRows_BlocksFollowTheirMeanForestColumn: B's epics sit left of A's
// in the forest, so B's block comes first, one empty column before A's,
// every node inside its own block.
func TestMapRows_BlocksFollowTheirMeanForestColumn(t *testing.T) {
	t.Parallel()
	in := threeBands()
	got := MapRows(&in)
	a, b := mapBlockOf(t, got.Blocks, "A"), mapBlockOf(t, got.Blocks, "B")
	assert.Less(t, b.ToCol, a.FromCol)
	assert.InDelta(t, 1.0, a.FromCol-b.ToCol-1, 0, "one empty column between blocks")
	for _, id := range in.IDs {
		blk, p := mapBlockOf(t, got.Blocks, in.Group[id]), got.Places[id]
		assert.True(t, blk.FromCol <= p.X && p.X <= blk.ToCol && blk.FromY <= p.Y && p.Y <= blk.ToY, "%s at %+v outside %+v", id, p, blk)
	}
}

// TestMapRows_ALayerIsCentredInItsBlock: A is two columns wide (layer 1
// holds two epics), so its lone epics of layers 0 and 2 sit half-way
// across it and layer 1's two side by side in forest order.
func TestMapRows_ALayerIsCentredInItsBlock(t *testing.T) {
	t.Parallel()
	in := threeBands()
	got := MapRows(&in)
	a := mapBlockOf(t, got.Blocks, "A")
	assert.InDelta(t, 1.0, a.ToCol-a.FromCol, 0)
	assert.InDelta(t, a.FromCol+0.5, got.Places["a0"].X, 0)
	assert.InDelta(t, a.FromCol, got.Places["a1"].X, 0)
	assert.InDelta(t, a.FromCol+1, got.Places["a2"].X, 0)
}

// TestMapRows_AWideLayerFoldsInsideItsOwnBlock: a layer of ten epics folds
// onto two sub-lines of a block eight columns wide, and its next layer
// starts below both; V beside it keeps its own shape, its layers one y
// apart, so the fold makes only W's block taller.
func TestMapRows_AWideLayerFoldsInsideItsOwnBlock(t *testing.T) {
	t.Parallel()
	nodes := []mapNode{{"w", "W", 0, 0}, {"v", "V", 0, 20}, {"v1", "V", 1, 20}}
	for i := range 10 {
		nodes = append(nodes, mapNode{fmt.Sprintf("w%d", i), "W", 1, float64(i)})
	}
	nodes = append(nodes, mapNode{"wz", "W", 2, 0}, mapNode{"vz", "V", 2, 20})
	in := mapInput(30, []LayoutBand{{Name: "W", From: 0, To: 2}, {Name: "V", From: 0, To: 2}}, nodes...)
	got := MapRows(&in)
	w := mapBlockOf(t, got.Blocks, "W")
	assert.InDelta(t, 7.0, w.ToCol-w.FromCol, 0)
	assert.InDelta(t, got.Places["w0"].Y+1, got.Places["w9"].Y, 0)
	assert.InDelta(t, got.Places["w9"].Y+1, got.Places["wz"].Y, 0)
	assert.InDelta(t, got.Places["v"].Y, got.Places["w"].Y, 0)
	assert.InDelta(t, got.Places["v1"].Y+1, got.Places["vz"].Y, 0, "V keeps its own shape")
	assert.InDelta(t, got.Places["wz"].Y+1, got.Height, 0, "the line as tall as W")
}

// TestMapRows_BlocksWrapInsideTheWidth: five two-column blocks sharing
// layers do not fit nine columns, so they wrap onto lines of three blocks
// and two, the lines centred on each other, no block past the width, the
// lines stacked down.
func TestMapRows_BlocksWrapInsideTheWidth(t *testing.T) {
	t.Parallel()
	var nodes []mapNode
	var bands []LayoutBand
	for i := range 5 {
		g := fmt.Sprintf("M%d", i)
		bands = append(bands, LayoutBand{Name: g, From: 0, To: 0})
		nodes = append(nodes, mapNode{g + "a", g, 0, float64(2 * i)}, mapNode{g + "b", g, 0, float64(2*i + 1)})
	}
	in := mapInput(9, bands, nodes...)
	got := MapRows(&in)
	require.Len(t, got.Rows, 1)
	assert.Equal(t, 2, got.Rows[0].Lines)
	for _, b := range got.Blocks {
		assert.True(t, b.FromCol >= 0 && b.ToCol < 9, "%+v", b)
	}
	m0, m2, m3 := mapBlockOf(t, got.Blocks, "M0"), mapBlockOf(t, got.Blocks, "M2"), mapBlockOf(t, got.Blocks, "M3")
	assert.Equal(t, []int{0, 0, 1}, []int{m0.Line, m2.Line, m3.Line})
	assert.InDelta(t, 0.0, m0.FromCol, 0, "the widest line starts the map")
	assert.InDelta(t, 1.5, m3.FromCol, 0, "a five-column line centred under an eight-column one")
	assert.Less(t, m0.ToY, m3.FromY)
}

// TestMapRows_ADividerSplitsEachGap: one divider per pair of neighbouring
// blocks of a line, half-way across the gap, from the line's header unit
// to its last y.
func TestMapRows_ADividerSplitsEachGap(t *testing.T) {
	t.Parallel()
	in := threeBands()
	got := MapRows(&in)
	require.Len(t, got.Dividers, 1)
	a, b := mapBlockOf(t, got.Blocks, "A"), mapBlockOf(t, got.Blocks, "B")
	d := got.Dividers[0]
	assert.InDelta(t, (b.ToCol+a.FromCol)/2, d.X, 0)
	assert.InDelta(t, got.Rows[0].FromY, d.FromY, 0)
	assert.InDelta(t, got.Rows[0].ToY, d.ToY, 0)
}

// TestMapRows_IsStable: one input lays out alike.
func TestMapRows_IsStable(t *testing.T) {
	t.Parallel()
	in := threeBands()
	first := MapRows(&in)
	for range 20 {
		assert.Equal(t, first, MapRows(&in))
	}
}

// TestMapRows_AVeryWideLayerWidensPastTheCap: a layer of 27 epics would
// take four sub-lines at eight columns, so its block widens to
// ceil(27/3) = 9 columns and folds onto three.
func TestMapRows_AVeryWideLayerWidensPastTheCap(t *testing.T) {
	t.Parallel()
	var nodes []mapNode
	for i := range 27 {
		nodes = append(nodes, mapNode{fmt.Sprintf("w%02d", i), "W", 0, float64(i)})
	}
	in := mapInput(30, []LayoutBand{{Name: "W", From: 0, To: 0}}, nodes...)
	got := MapRows(&in)
	w := mapBlockOf(t, got.Blocks, "W")
	assert.InDelta(t, 8.0, w.ToCol-w.FromCol, 0)
	assert.InDelta(t, 2.0, w.ToY-w.FromY, 0)
}

// TestMapRows_ALineSkipsTheLayersNoneOfItsBlocksTakes: X takes layers 0-3
// and Y only 2-3 of the same row; Y wraps onto a line of its own, which
// opens with its header unit and then Y's own layers, not the two empty
// layers above them.
func TestMapRows_ALineSkipsTheLayersNoneOfItsBlocksTakes(t *testing.T) {
	t.Parallel()
	in := mapInput(2, []LayoutBand{{Name: "X", From: 0, To: 3}, {Name: "Y", From: 2, To: 3}},
		mapNode{"x0", "X", 0, 0}, mapNode{"x1", "X", 1, 0}, mapNode{"x2", "X", 2, 0}, mapNode{"x3", "X", 3, 0},
		mapNode{"y2", "Y", 2, 10}, mapNode{"y3", "Y", 3, 10})
	got := MapRows(&in)
	y := mapBlockOf(t, got.Blocks, "Y")
	require.Equal(t, 1, y.Line)
	assert.InDelta(t, y.LineY+1, got.Places["y2"].Y, 0)
	assert.InDelta(t, y.LineY+2, got.Places["y3"].Y, 0)
	assert.InDelta(t, y.LineY+3, got.Height, 0)
}

// TestMapRows_ReportsTheWidthItLaysOut: threeBands' blocks take far less
// than the forest's twenty columns, so the map starts at column 0 and its
// width is its widest line, not the forest's.
func TestMapRows_ReportsTheWidthItLaysOut(t *testing.T) {
	t.Parallel()
	in := threeBands()
	got := MapRows(&in)
	lo, hi := got.Blocks[0].FromCol, got.Blocks[0].ToCol
	for _, b := range got.Blocks {
		lo, hi = min(lo, b.FromCol), max(hi, b.ToCol)
	}
	assert.InDelta(t, 0.0, lo, 0)
	assert.InDelta(t, hi+1, got.Width, 0)
	assert.Less(t, got.Width, in.Width)
}

// TestMapRows_BlocksOfARowStartLevel: B's epics depend on A's and sit on
// the row's later layers, yet B's block starts at the row's first y beside
// A's, keeping its own shape, and the row ends where its tallest block
// does.
func TestMapRows_BlocksOfARowStartLevel(t *testing.T) {
	t.Parallel()
	in := mapInput(20, []LayoutBand{{Name: "A", From: 0, To: 1}, {Name: "B", From: 1, To: 3}},
		mapNode{"a0", "A", 0, 0}, mapNode{"a1", "A", 1, 0},
		mapNode{"b2", "B", 2, 5}, mapNode{"b3", "B", 3, 5})
	got := MapRows(&in)
	require.Len(t, got.Rows, 1)
	a, b := mapBlockOf(t, got.Blocks, "A"), mapBlockOf(t, got.Blocks, "B")
	assert.InDelta(t, a.LineY+1, a.FromY, 0)
	assert.InDelta(t, a.FromY, b.FromY, 0, "both blocks start at the row's first y")
	assert.InDelta(t, got.Places["b2"].Y+1, got.Places["b3"].Y, 0, "B keeps its own shape")
	assert.InDelta(t, b.FromY, got.Places["b2"].Y, 0)
	assert.InDelta(t, max(a.ToY, b.ToY), got.Rows[0].ToY, 0)
	assert.InDelta(t, got.Rows[0].ToY+1, got.Height, 0)
}

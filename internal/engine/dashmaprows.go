package engine

import (
	"cmp"
	"math"
	"slices"
)

// The map's block geometry: the empty columns between two blocks of a
// line, the widest a block grows before its layers fold onto sub-lines,
// the sub-lines a fold aims at, and the y units a line keeps above its
// first layer for its header and its blocks' labels.
const (
	mapBlockGap   = 1.0
	mapFoldCap    = 8
	mapFoldLines  = 3
	mapHeaderRows = 1.0
)

// MapRowsInput is what MapRows lays out: IDs in file order, each node's
// Layer and forest column (Col), its Group (its milestone), the bands with
// their layers, and Width, the forest's width in columns.
type MapRowsInput struct {
	IDs   []string
	Layer map[string]int
	Col   map[string]float64
	Group map[string]string
	Bands []LayoutBand
	Width float64
}

// MapPlace is a node's final place on the map, in column (X) and row (Y)
// units.
type MapPlace struct {
	X float64
	Y float64
}

// MapBlock is one group's block in one row: the row and the line of it the
// block sits on, that line's header y (where the block's label goes), the
// layers its nodes take, the columns and y units its nodes take, inclusive,
// and its node count.
type MapBlock struct {
	Milestone string  `json:"milestone"`
	Row       int     `json:"row"`
	Line      int     `json:"line"`
	LineY     float64 `json:"line_y"`
	FromLayer int     `json:"from_layer"`
	ToLayer   int     `json:"to_layer"`
	FromCol   float64 `json:"from_col"`
	ToCol     float64 `json:"to_col"`
	FromY     float64 `json:"from_y"`
	ToY       float64 `json:"to_y"`
	Count     int     `json:"count"`
}

// MapRow is one row: the layers it spans, the y units it takes (its first
// line's header unit included), its lines, and how many distinct groups and
// nodes it holds.
type MapRow struct {
	FromLayer  int     `json:"from_layer"`
	ToLayer    int     `json:"to_layer"`
	FromY      float64 `json:"from_y"`
	ToY        float64 `json:"to_y"`
	Lines      int     `json:"lines"`
	Milestones int     `json:"milestones"`
	Count      int     `json:"count"`
}

// MapDivider is a vertical line through the middle of the gap between two
// blocks of one line, at X, from the line's header to its last y unit.
type MapDivider struct {
	Row   int     `json:"row"`
	Line  int     `json:"line"`
	X     float64 `json:"x"`
	FromY float64 `json:"from_y"`
	ToY   float64 `json:"to_y"`
}

// MapRowsLayout is MapRows' answer: each node's place, the blocks, rows
// and dividers, and the map's height in y units.
type MapRowsLayout struct {
	Places   map[string]MapPlace
	Blocks   []MapBlock
	Rows     []MapRow
	Dividers []MapDivider
	Height   float64
}

// mapBlock is a block being laid out: its group, its nodes per layer in
// forest-column order, its width, the sub-lines each layer takes, its
// mean forest column and the file index of its first node.
type mapBlock struct {
	group  string
	layers map[int][]string
	width  int
	lines  map[int]int
	mean   float64
	first  int
	x      float64
	count  int
}

// MapRows lays the plan out in rows of side-by-side milestone blocks.
// Bands that share a layer, directly or through another band, make one
// row. Each group with a node in a row is one block of it, the blocks in
// the order of their mean forest column, then of their first node. A
// block is as wide as its fullest layer, up to mapFoldCap; past it the
// block is mapFoldCap wide, or ceil(fullest/mapFoldLines) where that is
// wider, so its fullest layer folds onto two or three sub-lines, and every
// layer of it takes as many sub-lines as its nodes need. Inside
// a sub-line the nodes keep their forest-column order and are centred in
// the block. Blocks sit mapBlockGap columns apart and wrap onto another
// line of the row where the next would pass Width, each line centred in
// Width. Every line keeps mapHeaderRows above its first layer, and each
// layer of a line one y per sub-line its fullest block needs, so a layer
// keeps one y across the line; a layer none of the line's blocks takes
// takes no y in it. One input always lays out alike.
func MapRows(in *MapRowsInput) MapRowsLayout {
	file := make(map[string]int, len(in.IDs))
	for i, id := range in.IDs {
		file[id] = i
	}
	out := MapRowsLayout{Places: make(map[string]MapPlace, len(in.IDs))}
	y := 0.0
	for r, span := range mapRowSpans(in) {
		blocks := mapRowBlocks(in, file, span)
		row := MapRow{FromLayer: span[0], ToLayer: span[1], FromY: y, Milestones: len(blocks)}
		for li, line := range mapWrap(blocks, in.Width) {
			y = mapPlaceLine(in, &out, line, span, [2]int{r, li}, y)
			row.Lines++
		}
		for _, b := range blocks {
			row.Count += b.count
		}
		row.ToY = y - 1
		out.Rows = append(out.Rows, row)
	}
	out.Height = y
	return out
}

// mapRowSpans is the rows' layer spans in layer order: the bands' spans
// and each node's layer, merged wherever two share a layer.
func mapRowSpans(in *MapRowsInput) [][2]int {
	var spans [][2]int
	for _, b := range in.Bands {
		spans = append(spans, [2]int{b.From, b.To})
	}
	for _, id := range in.IDs {
		spans = append(spans, [2]int{in.Layer[id], in.Layer[id]})
	}
	slices.SortFunc(spans, func(a, b [2]int) int { return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1])) })
	var out [][2]int
	for _, s := range spans {
		if n := len(out); n > 0 && s[0] <= out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], s[1])
			continue
		}
		out = append(out, s)
	}
	return out
}

// mapRowBlocks is the blocks of the row spanning span, each group with a
// node in it one block, sized and ordered as MapRows says.
func mapRowBlocks(in *MapRowsInput, file map[string]int, span [2]int) []*mapBlock {
	byGroup := map[string]*mapBlock{}
	var blocks []*mapBlock
	for i, id := range in.IDs {
		l := in.Layer[id]
		if l < span[0] || l > span[1] {
			continue
		}
		b := byGroup[in.Group[id]]
		if b == nil {
			b = &mapBlock{group: in.Group[id], layers: map[int][]string{}, lines: map[int]int{}, first: i}
			byGroup[b.group] = b
			blocks = append(blocks, b)
		}
		b.layers[l] = append(b.layers[l], id)
		b.mean += in.Col[id]
		b.count++
	}
	for _, b := range blocks {
		b.mean /= float64(b.count)
		mapSizeBlock(in, file, b)
	}
	slices.SortStableFunc(blocks, func(a, b *mapBlock) int {
		return cmp.Or(cmp.Compare(a.mean, b.mean), cmp.Compare(a.first, b.first))
	})
	return blocks
}

// mapSizeBlock sorts each layer of b by forest column, then file order,
// and gives b its width and each layer its sub-lines.
func mapSizeBlock(in *MapRowsInput, file map[string]int, b *mapBlock) {
	fullest := 0
	for _, ids := range b.layers {
		slices.SortFunc(ids, func(p, q string) int { return cmp.Or(cmp.Compare(in.Col[p], in.Col[q]), cmp.Compare(file[p], file[q])) })
		fullest = max(fullest, len(ids))
	}
	b.width = fullest
	if fullest > mapFoldCap {
		b.width = max(mapFoldCap, (fullest+mapFoldLines-1)/mapFoldLines)
	}
	for l, ids := range b.layers {
		b.lines[l] = (len(ids) + b.width - 1) / b.width
	}
}

// mapWrap breaks blocks into lines, a block going onto a new line where it
// would pass width after the gap, and places each block of a line from
// column 0 of it.
func mapWrap(blocks []*mapBlock, width float64) [][]*mapBlock {
	var lines [][]*mapBlock
	cursor := 0.0
	for _, b := range blocks {
		w := float64(b.width)
		if n := len(lines); n > 0 && cursor+mapBlockGap+w <= width {
			b.x = cursor + mapBlockGap
			lines[n-1] = append(lines[n-1], b)
		} else {
			b.x = 0
			lines = append(lines, []*mapBlock{b})
		}
		cursor = b.x + w
	}
	return lines
}

// mapPlaceLine places one line of a row, the row's index and the line's in
// at, from y: its header unit, then each layer of span with the sub-lines
// its fullest block needs (none where no block of the line takes the
// layer), the line centred in in.Width. It records the
// line's blocks and dividers in out and returns the y after the line.
func mapPlaceLine(in *MapRowsInput, out *MapRowsLayout, line []*mapBlock, span, at [2]int, y float64) float64 {
	last := line[len(line)-1]
	shift := max(0, (in.Width-(last.x+float64(last.width)))/2)
	top := y
	y += mapHeaderRows
	layerY := map[int]float64{}
	for l := span[0]; l <= span[1]; l++ {
		layerY[l] = y
		lines := 0
		for _, b := range line {
			lines = max(lines, b.lines[l])
		}
		y += float64(lines)
	}
	for i, b := range line {
		blk := mapPlaceBlock(out, b, shift, layerY, at)
		blk.LineY = top
		out.Blocks = append(out.Blocks, blk)
		if i > 0 {
			prev := line[i-1]
			x := shift + (prev.x+float64(prev.width)+b.x)/2 - 0.5
			out.Dividers = append(out.Dividers, MapDivider{Row: at[0], Line: at[1], X: x, FromY: top, ToY: y - 1})
		}
	}
	return y
}

// mapPlaceBlock places b's nodes, shifted by shift, each layer from its y
// in layerY, and returns the block.
func mapPlaceBlock(out *MapRowsLayout, b *mapBlock, shift float64, layerY map[int]float64, at [2]int) MapBlock {
	mb := MapBlock{Milestone: b.group, Row: at[0], Line: at[1], FromLayer: math.MaxInt, ToLayer: -1, FromCol: shift + b.x, ToCol: shift + b.x + float64(b.width) - 1, FromY: math.Inf(1), ToY: math.Inf(-1), Count: b.count}
	for l, ids := range b.layers {
		mb.FromLayer, mb.ToLayer = min(mb.FromLayer, l), max(mb.ToLayer, l)
		for k, id := range ids {
			sub, pos := k/b.width, k%b.width
			inSub := min(b.width, len(ids)-sub*b.width)
			p := MapPlace{X: shift + b.x + float64(b.width-inSub)/2 + float64(pos), Y: layerY[l] + float64(sub)}
			out.Places[id] = p
			mb.FromY, mb.ToY = min(mb.FromY, p.Y), max(mb.ToY, p.Y)
		}
	}
	return mb
}

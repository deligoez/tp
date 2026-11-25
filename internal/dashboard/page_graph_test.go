package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPure_ACriticalEdgeIsDrawnWhateverItsClass is decision 2 of
// spec/backlog/dashboard-graph.md: an edge of the marked critical path is
// drawn even when its class is hidden, so Gate closure's default view, cross
// edges off, never shows the path with holes; a plain cross edge waits for
// the toggle, and the legend can still hide the critical path itself.
func TestPure_ACriticalEdgeIsDrawnWhateverItsClass(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const off = { showCross: false, hidden: [] };
out([
  P.edgeDrawn({ role: "cross", critical: true }, off),
  P.edgeDrawn({ role: "tree", critical: true }, { showCross: false, hidden: ["tree"] }),
  P.edgeDrawn({ role: "cross" }, off),
  P.edgeDrawn({ role: "cross" }, { showCross: true, hidden: [] }),
  P.edgeDrawn({ role: "tree" }, off),
  P.edgeDrawn({ role: "back" }, { showCross: false, hidden: ["back"] }),
  P.edgeDrawn({ role: "tree", any: true }, { showCross: false, hidden: ["any"] }),
  P.edgeDrawn({ role: "cross", critical: true }, { showCross: false, hidden: ["critical"] }),
]);`)
	assert.JSONEq(t, `[true, true, false, true, true, false, false, false]`, got)
}

// TestPure_TheGraphsEdgeLayersHoldEveryDrawnEdgeOnce: the card splits the
// edges it draws into its four layers, each drawn edge in exactly one, so
// the count it records on the card is the layout's: every edge with cross
// edges shown, and with them hidden every edge but the plain cross ones.
func TestPure_TheGraphsEdgeLayersHoldEveryDrawnEdgeOnce(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const links = [
  { role: "tree" }, { role: "cross" }, { role: "cross", critical: true }, { role: "back" },
  { role: "tree", any: true }, { role: "cross", any: true }, { role: "tree", critical: true, any: true },
];
const count = (v) => { const s = P.edgeLayers(links, v); return [s.cross.length, s.any.length, s.tree.length, s.crit.length, s.drawn]; };
out([count({ showCross: true, hidden: [] }), count({ showCross: false, hidden: [] })]);`)
	assert.JSONEq(t, `[[1, 2, 2, 2, 7], [0, 1, 2, 2, 5]]`, got)
}

// TestPure_TheGraphsPlaceRoundTripsThroughTheAddress: the card's mode, gate
// milestone, depth, cross-edge toggle and search go into the address as gv,
// gm, gd, cross and q, each only when it is not the default, and read back
// to the same place; a value outside its set reads as the default. The
// packing is the API's alone: the page neither writes nor reads pack.
func TestPure_TheGraphsPlaceRoundTripsThroughTheAddress(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const s = { mode: "scope", milestone: "A3", depth: 3, showCross: false, q: "cache" };
const p = new URLSearchParams(P.graphParams(s));
out([
  p.toString(),
  P.graphPlace((k) => p.get(k)),
  P.graphParams({ mode: "plan", milestone: "A3", depth: 2, showCross: true, q: "" }),
  P.graphPlace((k) => ({ gv: "map", gd: "9", pack: "tree", cross: "yes" })[k] || null),
]);`)
	assert.JSONEq(t, `[
		"gv=scope&gm=A3&gd=3&cross=0&q=cache",
		{"mode": "scope", "milestone": "A3", "depth": 3, "showCross": false, "q": "cache"},
		[],
		{"mode": "plan", "milestone": null, "depth": 2, "showCross": true, "q": ""}
	]`, got)
}

// TestPure_EnterStepsThroughTheMatches: Enter in the search moves to the
// next match and wraps to the first after the last, Shift+Enter to the
// previous, from no match yet (-1) to the first or the last.
func TestPure_EnterStepsThroughTheMatches(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.stepMatch(3, -1, false), P.stepMatch(3, 0, false), P.stepMatch(3, 2, false), P.stepMatch(3, -1, true), P.stepMatch(3, 0, true), P.stepMatch(0, -1, false)]);`)
	assert.JSONEq(t, `[0, 1, 0, 2, 2, -1]`, got)
}

// TestPure_TheChainFadesPastTwoStepsUnlessTheWholeChainIsAsked: the
// selection's chain is full within two steps and fades after, unless the
// legend's "show the whole chain" is on; withinTwo counts the near part.
func TestPure_TheChainFadesPastTwoStepsUnlessTheWholeChainIsAsked(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.chainAlpha(3, true), P.chainAlpha(9, true), P.chainAlpha(9, false), P.withinTwo(new Map([[1, 1], [2, 2], [3, 3], [4, 5]]))]);`)
	assert.JSONEq(t, `[255, 255, 70, 2]`, got)
}

// TestPure_TheThemeIsTheStoredChoiceElseTheSystems: a stored light or dark
// wins; anything else leaves the page to the system's setting (null).
func TestPure_TheThemeIsTheStoredChoiceElseTheSystems(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.pickTheme("dark"), P.pickTheme("light"), P.pickTheme("blue"), P.pickTheme(null)]);`)
	assert.JSONEq(t, `["dark", "light", null, null]`, got)
}

// TestPure_BandLabelsNeverCollide: a band label that would overlap one
// already placed is shortened to its name alone, and skipped when even
// that overlaps; labels are placed in the order given, each box its left
// edge x, its top y, its full width w and its short width sw.
func TestPure_BandLabelsNeverCollide(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const box = (x, y, w, sw) => ({ x, y, w, h: 16, sw });
out(P.placeLabels([box(100, 0, 100, 40), box(0, 0, 150, 60), box(40, 0, 100, 50), box(0, 40, 100, 40)]));`)
	assert.JSONEq(t, `[{"kind":"full","x":100,"y":0}, {"kind":"short","x":0,"y":0}, null, {"kind":"full","x":0,"y":40}]`, got)
}

// TestPure_ABandLabelNeverCoversANode: a label first tries its band's
// top-left corner, then the strip just above the band, full and then short
// at each; a node's box is an obstacle like a label already placed, and a
// label with no free place is left out (the band's name still shows on
// hover), never drawn over a node.
func TestPure_ABandLabelNeverCoversANode(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const lab = (x, y, w, sw, above) => ({ x, y, w, h: 20, sw, alts: [{ x, y: above }] });
const nodes = [{ x: 0, y: 0, w: 200, h: 28 }];
out(P.placeLabels([
  lab(0, 6, 120, 60, -24),
  lab(300, 6, 120, 60, -24),
  lab(0, 200, 120, 60, 170),
], nodes.concat([{ x: 0, y: 160, w: 400, h: 70 }])));`)
	assert.JSONEq(t, `[{"kind":"full","x":0,"y":-24}, {"kind":"full","x":300,"y":6}, null]`, got)
}

// TestPure_TheGlyphAtlasHoldsEveryCharacterANodeDraws: deck.gl draws only
// the characters its atlas was built from, so the atlas takes them from the
// text a node draws, not only from its id and title.
func TestPure_TheGlyphAtlasHoldsEveryCharacterANodeDraws(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(P.glyphsOf([{ label: "a", title: "", text: "a\n…" }], []).includes("…"));`)
	assert.Equal(t, "true", got)
}

// TestPure_ALongListFoldsPastItsFirstFew: the drawer shows a list's first
// n rows and folds the rest, but never folds a single row, which would take
// a "+1 more" line to hide one line.
func TestPure_ALongListFoldsPastItsFirstFew(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const l = (n) => Array.from({ length: n }, (_, i) => i);
out([P.foldAt(l(3), 5), P.foldAt(l(6), 5), P.foldAt(l(8), 5)]);`)
	assert.JSONEq(t, `[[[0,1,2],[]], [[0,1,2,3,4,5],[]], [[0,1,2,3,4],[5,6,7]]]`, got)
}

// TestPure_MarkdownDrawsADescriptionSafely: an epic's description is
// markdown: headings, lists, fenced code (a blank line inside the fence
// does not end it), bold, italic, code spans and http(s) links draw, and
// everything else is escaped text: a script, raw HTML, a javascript: link.
func TestPure_MarkdownDrawsADescriptionSafely(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(P.markdown([
  "## Purpose",
  "Make the **schema** dependable, *not* fast; see [the doc](https://x.dev/a?b=1&c=2).",
  "",
  "- one with `+"`code`"+`",
  "- two",
  "",
  "`+"```"+`",
  "a <b>",
  "",
  "b",
  "`+"```"+`",
  "",
  "<script>alert(1)</script> [bad](javascript:alert(1))",
].join("\n")));`)
	assert.Equal(t, `"<h3 class=\"sub\">Purpose</h3><p>Make the <b>schema</b> dependable, <i>not</i> fast; see <a href=\"https://x.dev/a?b=1&amp;c=2\" target=\"_blank\" rel=\"noopener noreferrer\">the doc</a>.</p>`+
		`<ul><li>one with <code>code</code></li><li>two</li></ul><pre>a &lt;b&gt;\n\nb</pre>`+
		`<p>&lt;script&gt;alert(1)&lt;/script&gt; [bad](javascript:alert(1))</p>"`, got)
}

// TestPure_HopsCountTheStepsFromTheSelection: the highlight of a
// selection's chain fades with distance, so the page counts each reachable
// node's steps from the selected one, breadth first, the selection itself
// not among them.
func TestPure_HopsCountTheStepsFromTheSelection(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const next = [[1, 2], [3], [3], [4], []];
out([[...P.hops(0, (v) => next[v])], P.chainAlpha(1, false), P.chainAlpha(2, false), P.chainAlpha(3, false), P.chainAlpha(9, false)]);`)
	assert.JSONEq(t, `[[[1, 1], [2, 1], [3, 2], [4, 3]], 255, 255, 170, 70]`, got)
}

// TestPure_ANodeSitsAtItsMapPlace: a node the layout gives a final x and y
// (the plan's map) sits there, x = 0 included; one without them sits at
// its forest column and layer.
func TestPure_ANodeSitsAtItsMapPlace(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.mapPlace({ col: 4, layer: 2, x: 0, y: 5 }), P.mapPlace({ col: 4, layer: 2 })]);`)
	assert.JSONEq(t, `[{"col": 0, "row": 5}, {"col": 4, "row": 2}]`, got)
}

// TestPure_TheMapOpensAGapAboveEachLineOfBlocks: the page draws the plan's
// map roomier than its units, a gap above every line of blocks (the
// blocks' line_y), so a row's labels have a strip of their own at fit; a
// place below two line tops sits two gaps lower, one above none of them
// none.
func TestPure_TheMapOpensAGapAboveEachLineOfBlocks(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const tops = P.mapLineTops([{ line_y: 4 }, { line_y: 0 }, { line_y: 4 }]);
out([tops, P.mapY(-1, tops, 10, 100), P.mapY(0, tops, 10, 100), P.mapY(3, tops, 10, 100), P.mapY(4, tops, 10, 100), P.mapY(6, tops, 10, 100)]);`)
	assert.JSONEq(t, `[[0, 4], -10, 100, 130, 240, 260]`, got)
}

// TestPure_ASelectionLightsOnlyItsOwnChain: selecting an epic draws the
// edges from what it waits on into it and from it out to what waits on
// it, and no other: an edge from an ancestor to a node off the chain, one
// from a descendant's other parent, and one between two nodes off the
// chain stay unlit, wherever the map places them.
func TestPure_ASelectionLightsOnlyItsOwnChain(t *testing.T) {
	t.Parallel()
	got := runPure(t, `// 0 -> 1 -> 2 (selected) -> 3 -> 4; 0 -> 5; 6 -> 3; 5 -> 6
const edges = [[0, 1], [1, 2], [2, 3], [3, 4], [0, 5], [6, 3], [5, 6]];
const preds = (v) => edges.filter((e) => e[1] === v).map((e) => e[0]);
const succs = (v) => edges.filter((e) => e[0] === v).map((e) => e[1]);
const anc = P.hops(2, preds), desc = P.hops(2, succs);
out(edges.map((e) => P.onChain(2, anc, desc, e[0], e[1])));`)
	assert.JSONEq(t, `[true, true, true, true, false, false, false]`, got)
}

// TestPure_ADrawerOverAGraphLeavesItsChainUndimmed: an epic's drawer, and
// a task's over the Steps graph or the Map, opens beside the graph that
// draws the selection's chain, so no backdrop dims it; a milestone's
// drawer and a task's over the Board, where no chain is drawn, keep it.
func TestPure_ADrawerOverAGraphLeavesItsChainUndimmed(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.drawerDims("epic", "steps"), P.drawerDims("task", "steps"), P.drawerDims("task", "map"),
P.drawerDims("milestone", "steps"), P.drawerDims("task", "board"), P.drawerDims(null, "steps")]);`)
	assert.JSONEq(t, `[false, false, false, true, true, false]`, got)
}

// TestPure_AViewCanAskForALargerFirstLabel: a view that names the size its
// labels first show at (the plan's map: 14 pixels, DESIGN.md's tiny type)
// draws no text below it; one that names none keeps the 9-pixel floor.
func TestPure_AViewCanAskForALargerFirstLabel(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.detailLevel(13.9, 14), P.detailLevel(14, 14), P.detailLevel(8.9), P.detailLevel(9)]);`)
	assert.JSONEq(t, `["far", "near", "far", "near"]`, got)
}

// TestPure_AnIdWithWordsWrapsBetweenThem: an id written in words breaks at
// the last space in the line's second half, the space dropped, before it
// would break inside a word; ids without spaces wrap as before.
func TestPure_AnIdWithWordsWrapsBetweenThem(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.wrapId("Quiet Harbour Signal-I", 18), P.wrapId("inventory-export-backfill", 20)]);`)
	assert.JSONEq(t, `[["Quiet Harbour", "Signal-I"], ["inventory-export-", "backfill"]]`, got)
}

// TestPure_AWrapNeverLeavesASuffixAlone: a trailing suffix group such as
// "-I", "-II" or "-AI-I" stays on its word, the break falling at the last
// space before that word; with no break left before it, the id keeps one
// line rather than a tail of three characters or fewer.
func TestPure_AWrapNeverLeavesASuffixAlone(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.wrapId("Alpha Beta Gamma-I", 17), P.wrapId("Delta Epsilon-II", 15), P.wrapId("Zeta Eta Theta-AI-I", 18), P.wrapId("abcdefghijklmnopq-II", 18), P.wrapId("abcdefghijklmnopqrstu", 20)]);`)
	assert.JSONEq(t, `[["Alpha Beta", "Gamma-I"], ["Delta", "Epsilon-II"], ["Zeta Eta", "Theta-AI-I"], ["abcdefghijklmnopq-II"], ["abcdefghijklmnopqrstu"]]`, got)
}

// TestPure_AShortIdKeepsItsHeadAndItsSuffix: a shortened id keeps as much
// of its head as fits before an ellipsis and its suffix group whole, so
// "-I" and "-II" siblings still differ; a suffix that leaves under three
// characters of head is cut with the rest, and an id that fits is left
// alone.
func TestPure_AShortIdKeepsItsHeadAndItsSuffix(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.shortId("Alpha Beta Gamma-II", 9), P.shortId("Alpha-I", 9), P.shortId("inventory-export", 9), P.shortId("Zeta Eta Theta-AI-II", 9)]);`)
	assert.JSONEq(t, `["Alpha…-II", "Alpha-I", "inventor…", "Zeta Eta…"]`, got)
}

// TestPure_AShortLabelShowsBeforeTheWholeOne: a view with a short label
// size draws it once that size reaches the minimum on screen and the whole
// id does not yet; without one it stays far.
func TestPure_AShortLabelShowsBeforeTheWholeOne(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.detailLevel(8, 14, 14), P.detailLevel(8, 14, 13.9), P.detailLevel(14, 14, 25), P.detailLevel(8, 14)]);`)
	assert.JSONEq(t, `["short", "far", "near", "far"]`, got)
}

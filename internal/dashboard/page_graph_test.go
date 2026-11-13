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

// TestPure_TheGlyphAtlasHoldsEveryCharacterAShortenedIdDraws: deck.gl
// draws only the characters its atlas was built from, so an id shortened
// with an ellipsis must put the ellipsis in the atlas, or the id reads as
// cut off at a hyphen.
func TestPure_TheGlyphAtlasHoldsEveryCharacterAShortenedIdDraws(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const lines = P.wrapId("inference-policy-group-model-review", 20);
const node = { label: "inference-policy-group-model-review", title: "", text: lines.join("\n"), short: lines[0] + "…" };
out(P.glyphsOf([node], []).includes("…"));`)
	assert.Equal(t, "true", got)
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

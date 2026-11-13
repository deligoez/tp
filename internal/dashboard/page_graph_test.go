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
// milestone, depth, packing, cross-edge toggle and search go into the
// address as gv, gm, gd, pack, cross and q, each only when it is not the
// default, and read back to the same place; a value outside its set reads
// as the default.
func TestPure_TheGraphsPlaceRoundTripsThroughTheAddress(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const s = { mode: "scope", milestone: "A3", depth: 3, pack: "tree", showCross: true, q: "cache" };
const p = new URLSearchParams(P.graphParams(s));
out([
  p.toString(),
  P.graphPlace((k) => p.get(k)),
  P.graphParams({ mode: "plan", milestone: "A3", depth: 2, pack: "rows", showCross: false, q: "" }),
  P.graphPlace((k) => ({ gv: "map", gd: "9", pack: "wide", cross: "yes" })[k] || null),
]);`)
	assert.JSONEq(t, `[
		"gv=scope&gm=A3&gd=3&pack=tree&cross=1&q=cache",
		{"mode": "scope", "milestone": "A3", "depth": 3, "pack": "tree", "showCross": true, "q": "cache"},
		[],
		{"mode": "plan", "milestone": null, "depth": 2, "pack": "rows", "showCross": false, "q": ""}
	]`, got)
}

// TestPure_HopsCountTheStepsFromTheSelection: the highlight of a
// selection's chain fades with distance, so the page counts each reachable
// node's steps from the selected one, breadth first, the selection itself
// not among them.
func TestPure_HopsCountTheStepsFromTheSelection(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const next = [[1, 2], [3], [3], [4], []];
out([[...P.hops(0, (v) => next[v])], P.chainAlpha(1), P.chainAlpha(2), P.chainAlpha(3), P.chainAlpha(9)]);`)
	assert.JSONEq(t, `[[[1, 1], [2, 1], [3, 2], [4, 3]], 255, 255, 170, 70]`, got)
}

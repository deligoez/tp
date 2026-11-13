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


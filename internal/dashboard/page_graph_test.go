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


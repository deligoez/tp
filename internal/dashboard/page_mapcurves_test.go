package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The epic map draws each edge as v1.27.0 did, a curve from a dependency to
// its dependent in deck.gl: routed square lines turned a plan of thousands
// of edges into a grid of long lines. Only the Steps panel routes its wires.
func TestPage_TheEpicMapDrawsItsEdgesAsCurves(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, `links.forEach((l) => { l.path = curve(nodes[l.s], nodes[l.d], l.role === "back"); });`)
	for _, s := range []string{"function routeEdge(", "pure.routeWire(", "P.wirePoints"} {
		assert.NotContains(t, page, s)
	}
	assert.Contains(t, page, "P.routeWire = function", "the Steps panel keeps its router")
}

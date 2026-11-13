package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPage_SearchEnterStepsAndKeepsFocusInTheSearch is the 1.17.0 review's
// H1: Enter steps through the matches with tpPure.stepMatch, Shift+Enter
// back, and the drawer a match opens leaves focus in the search, so the
// next Enter steps again instead of landing on the drawer's close button.
func TestPage_SearchEnterStepsAndKeepsFocusInTheSearch(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"G.matchIdx = pure.stepMatch(hits.length, G.matchIdx, ev.shiftKey);", "app.selectEpic(n.id, false, true);",
		"function selectEpic(id, centre, keepFocus) {", "if (!keepFocus) $(\"dclose\").focus({ preventScroll: true });"} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheToolbarHasNoPackingControl is the review's M4: the packing
// stays on the API (pack=), but the card has no control for it and draws
// rows.
func TestPage_TheToolbarHasNoPackingControl(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.NotContains(t, page, `id="gc-pack"`)
	for _, s := range []string{"G.pack", "PACK_TEXT", "graph.packing"} {
		assert.NotContains(t, script, s)
	}
}

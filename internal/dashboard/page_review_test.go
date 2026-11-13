package dashboard_test

import (
	"strings"
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

// TestPage_TheNeighbourhoodKeepsItsCentreAndTheSearchWorksUnprepared is
// the review's H2: closing the drawer in Neighbourhood keeps the centre
// epic; with nothing laid out the search runs over the export's epics; "/"
// reaches the graph's search whenever the Epics tab shows the card, laid
// out or not, and never an input of a hidden tab.
func TestPage_TheNeighbourhoodKeepsItsCentreAndTheSearchWorksUnprepared(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"const sel = app.selected() || G.centre || firstReady(g);", "G.centre = id;", "function exportMatches(", "const cardShown = () => !card.hidden && !$(\"view-epics\").hidden;"} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, script, "!!G.prepared && !$(\"view-epics\").hidden")
}

// TestPage_AFailedLayoutClearsTheCanvasAndRecordsZero is the review's
// R1-L2: a layout fetch that fails clears deck.gl's layers and records 0
// drawn for the mode, so the canvas never shows the previous mode's picture
// under the error.
func TestPage_AFailedLayoutClearsTheCanvasAndRecordsZero(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "if (G.error) {\n      clearCanvas();\n      say(G.error.error);")
	assert.Contains(t, script, "G.drawing = { mode: G.mode, nodes: 0, edges: 0 };\n    recordDrawn();\n    if (G.deck) G.deck.setProps({ layers: [] });")
}

// TestPage_TheNeighbourhoodOpensWhereIdsRead is the review's M1: the
// neighbourhood opens near enough that an epic's id is drawn whole, at 12
// pixels since 1.17.1's levels of detail.
func TestPage_TheNeighbourhoodOpensWhereIdsRead(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "const NEAR_ZOOM = 0.13;")
}

// TestPage_TheCanvasIsNoTabStop is the review's M9: deck.gl's canvas takes
// tabIndex -1, so the canvas box is the card's one tab stop.
func TestPage_TheCanvasIsNoTabStop(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `$("gc-canvas").querySelectorAll("canvas").forEach((c) => { c.tabIndex = -1; });`)
}

// TestPage_MilestonesDrawCrossEdgesLighterThanTheSpine is the review's H3
// at the page: the card asks tpPure.crossShown, so Milestones draws its
// cross edges by default, as every mode does since 1.17.1, and K toggles
// the one flag.
func TestPage_MilestonesDrawCrossEdgesLighterThanTheSpine(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"pure.crossShown(G)", "showCross: true", "function toggleCross() {"} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheLegendCountsNodesAndTheNearChain is the review's M2 and M3:
// the marks' counts are labelled as the nodes drawn, conditions included,
// one mark each; the selection's counts say how many lie within two steps,
// and a toggle shows the whole chain unfaded.
func TestPage_TheLegendCountsNodesAndTheNearChain(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.NotContains(t, script, "graph.legendNodes", "1.17.1: the marks carry their counts without a heading")
	for _, s := range []string{`data-lmark="`, `t("graph.near", { n: P.desc.size, near: pure.withinTwo(P.desc) })`, `data-lwhole`, "pure.chainAlpha(stepsOf(P, n.i), G.whole)"} {
		assert.Contains(t, script, s)
	}
}

// TestPage_BandLabelsAreShortenedOrLeftOutWhereTheyWouldCollide is the
// review's L1: each frame places the band labels through tpPure.placeLabels
// in screen pixels, so two never overlap.
func TestPage_BandLabelsAreShortenedOrLeftOutWhereTheyWouldCollide(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "pure.placeLabels(")
}

// TestPage_TheThemeToggleSitsInTheHeaderAndPersists is the review's M5: the
// Light/Dark toggle is the page's, in its header, kept in localStorage
// behind a try, and without a stored choice the system's setting rules.
func TestPage_TheThemeToggleSitsInTheHeaderAndPersists(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	header := page[strings.Index(page, `<header class="top">`):strings.Index(page, "</header>")]
	assert.Contains(t, header, `id="theme"`)
	assert.NotContains(t, page, `id="gc-theme"`)
	for _, s := range []string{`const THEME_KEY = "tp-dashboard-theme";`, "tpPure.pickTheme(storedTheme())", "try { localStorage.setItem(THEME_KEY", `new Event("tp-theme")`} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheToolbarStaysOneLineAndTheCountSitsBelow is the review's M6:
// the count line sits under the toolbar, not in it, the closure's described
// count moves into the card's info popover, and the toolbar does not wrap
// at desktop widths.
func TestPage_TheToolbarStaysOneLineAndTheCountSitsBelow(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	toolbar := page[strings.Index(page, `<div class="toolbar">`):strings.Index(page, `<p class="gcount`)]
	assert.NotContains(t, toolbar, `id="gc-n"`)
	assert.Contains(t, page, `<p class="gcount sm muted num" id="gc-n" aria-live="polite"></p>`)
	assert.Contains(t, page, "@container (min-width: 1040px) { .gcard .toolbar { flex-wrap: nowrap; } }")
	assert.Contains(t, script, "INFO.graph = () =>")
	assert.NotContains(t, script, `"; " + pure.closureDescribed(`)
}

// TestPage_TheHelpIsAShortKeyListForTheMode is the review's M7: the
// legend's help is a short list of keys, worded per mode, and the search's
// placeholder names what it finds in that mode.
func TestPage_TheHelpIsAShortKeyListForTheMode(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{`"graph.help.milestones"`, `"graph.help.epics"`, `"graph.searchMilestone": "Find a milestone  /"`, `$("gc-q").placeholder = t(`} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheCanvasLabelNamesEveryKey is the review's L8: the canvas's
// aria-label names K and Esc too.
func TestPage_TheCanvasLabelNamesEveryKey(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `"graph.canvasLabel": "Graph canvas: the arrow keys pan, + and − zoom, F fits, K shows or hides cross edges, Esc clears the selection.`)
}

// TestPage_TheChainTextShowsDirectEdgesFirst is the review's M10: the
// chain as text lists the direct edges and folds the deeper ones into a
// details element, so a long chain is not thousands of tab stops.
func TestPage_TheChainTextShowsDirectEdgesFirst(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"function chainHTML(", `'<details class="deeper"><summary>'`, `t("chain.deeper", { n: deeper.length })`} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheNowListsKeepATitleColumnBesideTheDrawer is the drawer squeeze
// the i18n pass saw: a now list narrower than its row needs stacks the
// count under the title instead of crushing the title to one letter.
func TestPage_TheNowListsKeepATitleColumnBesideTheDrawer(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, ".now > .card { container-type: inline-size; }")
	assert.Contains(t, page, "@container (max-width: 380px) { .row { grid-template-columns: auto minmax(0, 1fr); } .row .m { grid-column: 2; white-space: normal; } }")
	assert.NotContains(t, page, "#map-q", "R1-L6: no rule styles the retired map search")
}

// TestPage_WithoutWebGL2TheControlsHide is the review's L3: without WebGL2
// the card hides its controls and legend and shows its message alone.
func TestPage_WithoutWebGL2TheControlsHide(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `card.classList.add("nogl");`)
	assert.Contains(t, page, ".gcard.nogl .toolbar > :not(h2), .gcard.nogl .legend, .gcard.nogl .gcount { display: none; }")
}

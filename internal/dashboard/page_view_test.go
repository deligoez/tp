package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPure_TheWheelZoomsWithAModifierPansInFullScreenAndElseScrollsThePage
// is 1.17.1's input rule, with no trackpad-or-mouse guess: Ctrl or ⌘ with
// the wheel, which is also how Chrome and Firefox report a pinch, zooms
// everywhere; the plain wheel, a mouse's or two fingers', pans the graph in
// full screen and scrolls the page outside it.
func TestPure_TheWheelZoomsWithAModifierPansInFullScreenAndElseScrollsThePage(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, false),
  P.wheelAction({ metaKey: true, deltaY: 3 }, true),
  P.wheelAction({ deltaY: 40 }, false),
  P.wheelAction({ deltaY: 40 }, true),
  P.wheelAction({ deltaX: 12 }, true),
]);`)
	assert.JSONEq(t, `["zoom", "zoom", "page", "pan", "pan"]`, got)
}

// TestPure_AWheelsDeltaIsReadInPixels: a wheel's delta counts pixels, lines
// of 16 pixels or pages of the canvas's height, by its deltaMode; the page
// reads every one as pixels before it zooms or pans.
func TestPure_AWheelsDeltaIsReadInPixels(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.wheelDelta({ deltaMode: 0, deltaX: 2, deltaY: -7 }, 600),
  P.wheelDelta({ deltaMode: 1, deltaX: 0, deltaY: 3 }, 600),
  P.wheelDelta({ deltaMode: 2, deltaX: 1, deltaY: -1 }, 600),
  P.wheelDelta({ deltaY: 5 }, 600),
]);`)
	assert.JSONEq(t, `[[2, -7], [0, 48], [600, -600], [0, 5]]`, got)
}

// TestPure_AZoomStepIsExponentialAndClampedPerEvent: the zoom is a power of
// two, so a step adds to it: a pinch's small deltas of 1 to 10 move it a
// little each, a mouse's 100 a full doubling, and no one event zooms by
// more than 2× either way. Safari's pinch reports a running scale, and the
// step is the log of its change since the last event, clamped the same way.
func TestPure_AZoomStepIsExponentialAndClampedPerEvent(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (x) => Math.round(x * 1000) / 1000;
out([
  r(P.wheelZoom(-4)), r(P.wheelZoom(4)), P.wheelZoom(-100), P.wheelZoom(-2000), P.wheelZoom(2000),
  P.pinchZoom(1, 2), P.pinchZoom(1, 16), P.pinchZoom(2, 1), r(P.pinchZoom(1, 1.1)), P.pinchZoom(0, 1), P.pinchZoom(1, 0),
]);`)
	assert.JSONEq(t, `[0.04, -0.04, 1, 1, -1, 1, 1, -1, 0.138, 0, 0]`, got)
}

// TestPage_TheCanvasTakesTheWheelAndSafarisPinchBeforeDeckGL: the canvas
// box listens in the capture phase, ahead of deck.gl's own canvas, with a
// listener that may cancel the page's scroll; Safari's gesture events zoom
// by the pinch's scale and are cancelled only on the canvas, where they are
// heard; the help says the rule in one sentence.
func TestPage_TheCanvasTakesTheWheelAndSafarisPinchBeforeDeckGL(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"pure.wheelAction(ev, card.classList.contains(\"fs\"))", "pure.wheelDelta(ev, ", "pure.wheelZoom(", "{ passive: false, capture: true }",
		`"gesturestart"`, `"gesturechange"`, "pure.pinchZoom(G.pinch, ev.scale)", `"graph.help.input": "A pinch, or Ctrl or ⌘ with the wheel, zooms; in full screen the wheel pans, outside it the page scrolls."`} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, script, "wheelZooms")
}

// TestPure_CrossEdgesAreDrawnByDefaultAndTheAddressKeepsOnlyOff: since
// 1.17.1 every mode draws cross edges until K hides them, one flag for all
// modes, and the address carries cross=0 only when they are hidden; an old
// link's cross=1 reads as the default.
func TestPure_CrossEdgesAreDrawnByDefaultAndTheAddressKeepsOnlyOff(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const on = { mode: "plan", milestone: null, depth: 2, showCross: true, q: "" };
out([
  P.crossShown({ mode: "plan" }), P.crossShown({ mode: "milestones" }), P.crossShown({ mode: "scope", showCross: false }), P.crossShown({ mode: "milestones", showCross: false }),
  P.graphParams(on), P.graphParams({ ...on, showCross: false }),
  P.graphPlace((k) => ({ cross: "0" })[k] || null).showCross, P.graphPlace(() => null).showCross, P.graphPlace((k) => ({ cross: "1" })[k] || null).showCross,
]);`)
	assert.JSONEq(t, `[true, true, false, false, [], [["cross", "0"]], false, true, true]`, got)
}

// TestPure_ALongEdgeSpansThreeLayersOrEightColumns: an edge whose ends lie
// three layers or more apart, or eight columns or more either way, is long,
// and the card draws it faded beneath the others.
func TestPure_ALongEdgeSpansThreeLayersOrEightColumns(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.edgeLong(1, 0), P.edgeLong(2, 7), P.edgeLong(3, 0), P.edgeLong(0, 8), P.edgeLong(1, -9), P.edgeLong(-3, 0)]);`)
	assert.JSONEq(t, `[false, false, true, true, true, true]`, got)
}

// TestPure_TheEdgeLayersRunCrossLongTreeTreeCritical: the card's edge
// layers run bottom to top as plain cross edges, any-of edges, long tree
// edges, short tree edges, the critical path; every drawn edge sits in one.
// With cross edges hidden, the cross edges of the selection's own chain
// still draw.
func TestPure_TheEdgeLayersRunCrossLongTreeTreeCritical(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const links = [
  { role: "tree" }, { role: "tree", long: true }, { role: "back", long: true }, { role: "cross" }, { role: "cross", long: true, mine: true },
  { role: "tree", critical: true, long: true }, { role: "tree", any: true, long: true },
];
const count = (v) => { const s = P.edgeLayers(links, v); return [s.cross.length, s.any.length, s.long.length, s.tree.length, s.crit.length, s.drawn]; };
out([count({ showCross: true, hidden: [] }), count({ showCross: false, hidden: [] }), count({ showCross: false, hidden: [], own: (l) => !!l.mine })]);`)
	assert.JSONEq(t, `[[2, 1, 2, 1, 1, 7], [0, 1, 2, 1, 1, 5], [1, 1, 2, 1, 1, 6]]`, got)
}

// TestPure_TheLegendCardStartsCollapsedInFullScreenAndKeepsTheChoice: the
// legend and keys card is open on the page and collapsed in full screen
// until the viewer chooses, and each place keeps its own choice ("1" open,
// "0" collapsed); anything else stored reads as no choice.
func TestPure_TheLegendCardStartsCollapsedInFullScreenAndKeepsTheChoice(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.legendOpen(null, false), P.legendOpen(null, true), P.legendOpen("1", true), P.legendOpen("0", false), P.legendOpen("yes", true), P.legendKey(false), P.legendKey(true)]);`)
	assert.JSONEq(t, `[true, false, true, false, false, "tp-dashboard-legend", "tp-dashboard-legend-fs"]`, got)
}

// TestPage_FullScreenFloatsTheToolbarAndTheLegendOverTheCanvas: in full
// screen the canvas takes the card's whole height; the toolbar floats over
// it as one pill-shaped line without the card's title, the count line and
// the long help hide, and the legend and keys float as one collapsible card
// at the bottom left that L opens and closes, its state kept in this
// browser behind a try.
func TestPage_FullScreenFloatsTheToolbarAndTheLegendOverTheCanvas(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		`<section class="glegend" id="gc-legendcard">`, `id="gc-lg" aria-expanded="true" aria-controls="gc-legend"`,
		"body.mapfs .gcard.fs .toolbar { position: absolute;", "border-radius: 999px;", "flex-wrap: nowrap;",
		"body.mapfs .gcard.fs .toolbar h2, body.mapfs .gcard.fs .toolbar .spacer, body.mapfs .gcard.fs .gcount, body.mapfs .gcard.fs .legend .help, body.mapfs .gcard.fs .chaintext { display: none; }",
		"body.mapfs .gcard.fs .glegend { position: absolute; left: 12px; bottom: 12px;",
		".glegend.closed .legend { display: none; }",
	} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{"pure.legendOpen(", "pure.legendKey(", `ev.key === "l" || ev.key === "L"`, "try { localStorage.setItem(pure.legendKey(", `new Event("tp-fs")`} {
		assert.Contains(t, script, s)
	}
}

// TestPure_TheThemeIsAutoUnlessLightOrDarkIsStored: the header's theme
// control reads Auto, Light or Dark; only an explicit Light or Dark is
// stored, and choosing Auto clears the stored value, so the page follows
// the system again.
func TestPure_TheThemeIsAutoUnlessLightOrDarkIsStored(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.themeChoice(null), P.themeChoice("dark"), P.themeChoice("light"), P.themeChoice("blue"), P.themeStore("auto"), P.themeStore("light"), P.themeStore("dark"), P.themeStore("x")]);`)
	assert.JSONEq(t, `["auto", "dark", "light", "auto", null, "light", "dark", null]`, got)
}

// TestPage_TheThemeControlFollowsTheSystemLiveOnAuto: the header holds
// three buttons, Auto, Light and Dark, worded in both languages; Auto
// removes the stored choice; a change of the system's setting tells the
// graph, which bumps its theme version so deck.gl draws every colour again
// without a reload.
func TestPage_TheThemeControlFollowsTheSystemLiveOnAuto(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	header := page[strings.Index(page, `<header class="top">`):strings.Index(page, "</header>")]
	for _, s := range []string{`data-theme-pick="auto"`, `data-theme-pick="light"`, `data-theme-pick="dark"`, `data-i18n="theme.auto"`} {
		assert.Contains(t, header, s)
	}
	for _, s := range []string{"tpPure.themeStore(", "localStorage.removeItem(THEME_KEY)", `SYSTEM_DARK.addEventListener("change", () => applyTheme(tpPure.pickTheme(storedTheme())))`, "G.themeVer++", `"theme.auto": "Auto"`} {
		assert.Contains(t, script, s)
	}
}

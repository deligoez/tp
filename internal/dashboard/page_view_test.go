package dashboard_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPure_TheWheelZoomsWithAModifierPansInFullScreenAndElseScrollsThePage
// is 1.17.1's input rule, with no trackpad-or-mouse guess: Ctrl or ⌘ with
// the wheel, which is also how Chrome and Firefox report a pinch, zooms
// on the canvas; the plain wheel, a mouse's or two fingers', pans the graph
// in full screen and scrolls the page outside it. In full screen a zoom
// over the card's floating toolbar or legend zooms the graph too, and one
// anywhere else, the drawer beside it, is cancelled: a pinch that reached
// the browser there zoomed the page, and with the graph refitted to its
// smaller box only the labels grew (the 1.17.1 pinch report).
func TestPure_TheWheelZoomsWithAModifierPansInFullScreenAndElseScrollsThePage(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, false, "canvas"),
  P.wheelAction({ metaKey: true, deltaY: 3 }, true, "canvas"),
  P.wheelAction({ deltaY: 40 }, false, "canvas"),
  P.wheelAction({ deltaY: 40 }, true, "canvas"),
  P.wheelAction({ deltaX: 12 }, true, "canvas"),
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, true, "card"),
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, true, "page"),
  P.wheelAction({ deltaY: 40 }, true, "card"),
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, false, "card"),
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, false, "page"),
]);`)
	assert.JSONEq(t, `["zoom", "zoom", "page", "pan", "pan", "zoom", "block", "page", "page", "page"]`, got)
}

// TestPure_SafarisPinchZoomsTheCanvasAndInFullScreenNeverThePage: Safari's
// gesture events zoom the graph on the canvas, and in full screen anywhere
// over the card; elsewhere in full screen they are cancelled, and outside
// it a pinch off the canvas zooms the page as Safari does.
func TestPure_SafarisPinchZoomsTheCanvasAndInFullScreenNeverThePage(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.gestureAction(false, "canvas"), P.gestureAction(true, "canvas"),
  P.gestureAction(true, "card"), P.gestureAction(true, "page"),
  P.gestureAction(false, "card"), P.gestureAction(false, "page"),
]);`)
	assert.JSONEq(t, `["zoom", "zoom", "zoom", "block", "page", "page"]`, got)
}

// TestPure_APageZoomDoesNotRefitTheGraph: the graph at its fit refits when
// its box changes size on the screen, in device pixels, as full screen, the
// drawer or the window change it; a page zoom shrinks the box in CSS
// pixels by the very factor it raises devicePixelRatio, so the box covers
// the same screen and the graph is left to zoom with the page. Refitting
// there kept the drawing's size while every label in pixels grew.
func TestPure_APageZoomDoesNotRefitTheGraph(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const at = { w: 1440, h: 900, dpr: 2 };
out([
  P.resizeRefits(null, at),
  P.resizeRefits(at, { w: 1440, h: 900, dpr: 2 }),
  P.resizeRefits(at, { w: 960, h: 600, dpr: 3 }),
  P.resizeRefits(at, { w: 1309, h: 818, dpr: 2.2 }),
  P.resizeRefits(at, { w: 1000, h: 900, dpr: 2 }),
  P.resizeRefits(at, { w: 1440, h: 700, dpr: 2 }),
  P.resizeRefits(at, { w: 1440, h: 900, dpr: 1 }),
]);`)
	assert.JSONEq(t, `[true, false, false, false, true, true, true]`, got)
}

// TestPure_ThePillShortensItsWordsOnlyWhenItOverflows: the full-screen
// toolbar keeps every word whole while it fits; only a pill wider than the
// room it has (beside an open drawer, on a narrow screen) shortens them.
func TestPure_ThePillShortensItsWordsOnlyWhenItOverflows(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.pillTight(900, 1400),
  P.pillTight(1400, 1400),
  P.pillTight(1400.6, 1400),
  P.pillTight(1035, 974),
  P.pillTight(0, 0),
]);`)
	assert.JSONEq(t, `[false, false, false, true, false]`, got)
}

// TestPure_TheCameraKeepsOneZoom: deck.gl's controller hands back a view
// state carrying zoomX and zoomY, and OrthographicView projects positions by
// those while it sizes text by zoom; a later zoom that changed zoom alone grew
// the labels and left the drawing where it was. The page keeps one zoom.
func TestPure_TheCameraKeepsOneZoom(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.cameraOf({ width: 800, height: 600, target: [10, 20, 0], zoom: -2, zoomX: -3, zoomY: -3, zoomAxis: "all", minZoomX: -8, maxZoomX: 3, maxBounds: null }),
  P.cameraOf({ target: [1, 2, 0], zoom: [-1, -1] }),
  P.cameraOf({ target: [1, 2], zoom: 0.5, minZoom: -8, maxZoom: 3 }),
]);`)
	assert.JSONEq(t, `[
  {"target": [10, 20, 0], "zoom": -2, "minZoom": -8, "maxZoom": 3},
  {"target": [1, 2, 0], "zoom": -1, "minZoom": -8, "maxZoom": 3},
  {"target": [1, 2, 0], "zoom": 0.5, "minZoom": -8, "maxZoom": 3}
]`, got)
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
// two, so a step adds to it. One pixel of wheel is 1/(100 ln 2), which
// makes Chrome's pinch, whose deltas undercount the fingers' spread, track
// them one to one and a mouse's notch of 100 zoom by 1.41×; no one event
// zooms by more than that, either way. Safari's pinch reports a running
// scale, and the step is the log of its change since the last event,
// limited the same way (measured by the 1.17.1 audit).
func TestPure_AZoomStepIsExponentialAndClampedPerEvent(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (x) => Math.round(x * 1000) / 1000;
out([
  r(P.wheelZoom(-4)), r(P.wheelZoom(4)), P.wheelZoom(-100), P.wheelZoom(-2000), P.wheelZoom(2000),
  P.pinchZoom(1, 2), P.pinchZoom(1, 16), P.pinchZoom(2, 1), r(P.pinchZoom(1, 1.1)), P.pinchZoom(0, 1), P.pinchZoom(1, 0),
]);`)
	assert.JSONEq(t, `[0.058, -0.058, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, 0.138, 0, 0]`, got)
}

// TestPage_TheCanvasTakesTheWheelAndSafarisPinchBeforeDeckGL: the canvas
// box listens in the capture phase, ahead of deck.gl's own canvas, with a
// listener that may cancel the page's scroll; Safari's gesture events zoom
// by the pinch's scale through the same rule by place; in full screen the
// document hears what lands off the canvas, and a resize refits only when
// the box changed on the screen; the help says the rule in one sentence.
func TestPage_TheCanvasTakesTheWheelAndSafarisPinchBeforeDeckGL(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, "body.mapfs { touch-action: pan-x pan-y; }")
	for _, s := range []string{"pure.wheelAction(ev, isFull(), place)", "pure.wheelDelta(ev, ", "pure.wheelZoom(", "{ passive: false, capture: true }",
		`"gesturestart"`, `"gesturechange"`, "pure.pinchZoom(G.pinch, ev.scale)", "pure.gestureAction(isFull(), place)",
		"listenOffCanvas(isFull())", "pure.resizeRefits(boxSize, next)",
		`"graph.help.input": "A pinch, or Ctrl or ⌘ with the wheel, zooms; in full screen the wheel pans, outside it the page scrolls."`} {
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

// TestPure_ATranslatorWordsAListAsBullets: a key whose text is an array of
// strings is a list of bullets; t.list gives each as markup, the
// dictionary's own tags kept and every value it fills escaped, a plain
// string as a list of one and a missing key as itself; t joins the bullets
// for a reader that wants one line.
func TestPure_ATranslatorWordsAListAsBullets(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const t = P.translator({ l: ["a {x}", "<kbd>K</kbd> b"], s: "one" }, "en");
out([t.list("l", { x: "<b>" }), t.list("s"), t.list("missing"), t("l", { x: "y" })]);`)
	assert.JSONEq(t, `[["a &lt;b&gt;", "<kbd>K</kbd> b"], ["one"], ["missing"], "a y <kbd>K</kbd> b"]`, got)
}

// TestPage_EveryInfoIsAShortBulletList: every info popover's words are an
// array in both languages, as many in Turkish as in English, kept to the
// audit's budget: a lead of at most 25 words, at most 6 bullets of at most
// 20 words, and a source line (From …, Kaynak: …) of at most 25; their only
// markup is kbd, code and b.
func TestPage_EveryInfoIsAShortBulletList(t *testing.T) {
	t.Parallel()
	en := dictionary(t, "en")
	tr := dictionary(t, "tr")
	tag := regexp.MustCompile(`<[^>]*>`)
	infos := 0
	for key := range en {
		if !strings.HasPrefix(key, "info.") {
			continue
		}
		infos++
		lists := map[string][]string{}
		for lang, d := range map[string]map[string]json.RawMessage{"en": en, "tr": tr} {
			var bullets []string
			if !assert.NoError(t, json.Unmarshal(d[key], &bullets), "%s %s is an array of strings", lang, key) {
				continue
			}
			if !assert.NotEmpty(t, bullets, "%s %s", lang, key) {
				continue
			}
			body := len(bullets) - 1
			if last := bullets[len(bullets)-1]; len(bullets) > 1 && (strings.HasPrefix(last, "From ") || strings.HasPrefix(last, "Kaynak")) {
				body--
			}
			assert.LessOrEqual(t, body, 6, "%s %s holds at most 6 bullets after its lead", lang, key)
			for i, b := range bullets {
				words := len(strings.Fields(tag.ReplaceAllString(b, "")))
				limit := 20
				if i == 0 || (i == len(bullets)-1 && i > body) {
					limit = 25
				}
				assert.LessOrEqual(t, words, limit, "%s %s: item %d stays within %d words: %q", lang, key, i, limit, b)
				for _, m := range tag.FindAllString(b, -1) {
					assert.Contains(t, []string{"<kbd>", "</kbd>", "<code>", "</code>", "<b>", "</b>"}, m, "%s %s", lang, key)
				}
			}
			lists[lang] = bullets
		}
		assert.Len(t, lists["tr"], len(lists["en"]), "%s holds as many bullets in Turkish as in English", key)
	}
	assert.Positive(t, infos)
}

// TestPage_AnInfoPopoverIsAHeadingAndABulletList: an (i) opens a popover
// named by its heading (aria-labelledby) that lists its bullets, at most
// about 420 pixels wide and scrolling when tall; focus moves to its one ×,
// and Esc or × closes it and gives focus back to the (i), Esc stopping
// there so an open drawer stays open.
func TestPage_AnInfoPopoverIsAHeadingAndABulletList(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{`'<div class="pophead"><h3 id="pop-h">' + esc(infoTitle(btn)) + '</h3><button type="button" class="popx" id="pop-x"`,
		`"<ul>" + items.map((s) => "<li>" + s + "</li>").join("") + "</ul>";`, `pop.setAttribute("aria-labelledby", "pop-h");`, `$("pop-x").focus({ preventScroll: true });`,
		"if (back && was && was.isConnected) was.focus({ preventScroll: true });", "ev.stopImmediatePropagation();\n    closeInfo(true);"} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, ".pop { position: absolute; z-index: var(--z-tooltip); max-width: min(420px, calc(100vw - 32px)); max-height: min(70vh, 560px); overflow-y: auto;")
}

// TestPure_TheDetailFollowsTheOnScreenFontSize: below 9 pixels on screen
// a node draws no text (far), and from 9 its whole id (near).
func TestPure_TheDetailFollowsTheOnScreenFontSize(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.detailLevel(4), P.detailLevel(8.9), P.detailLevel(9), P.detailLevel(11.9), P.detailLevel(12), P.detailLevel(30)]);`)
	assert.JSONEq(t, `["far", "far", "near", "near", "near", "near"]`, got)
}

// TestPure_ALongIdWrapsWholeInsideItsBox: an id longer than a line breaks
// after the last ".", "-" or ":" in the line's second half, else at the
// line's end, and is never cut short. Up to two lines of the box's width
// draw at its font; a longer id takes the line width whose wrap draws
// largest in the same box, the font cut to the longest line's width and to
// two lines' height.
func TestPure_ALongIdWrapsWholeInsideItsBox(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.fitId("api-gw-replay", 20), P.fitId("inventory-export-backfill", 20), P.fitId("abcdefghijklmnopqrstuvwxyz", 10), P.fitId("a.b:c-very-long-tail-of-the-identifier-here", 12)]);`)
	assert.JSONEq(t, `[
  {"lines": ["api-gw-replay"], "fit": 1},
  {"lines": ["inventory-export-", "backfill"], "fit": 1},
  {"lines": ["abcdefghijklm", "nopqrstuvwxyz"], "fit": 0.7692},
  {"lines": ["a.b:c-very-long-", "tail-of-the-", "identifier-here"], "fit": 0.6667}
]`, got)
}

// TestPure_ASmallGraphFitsNoFurtherOutThanItsLabelsRead: a gate closure, a
// neighbourhood, or any layout under 200 nodes fits no further out than
// where a label is 9 pixels on screen; a large plan may fit further out.
func TestPure_ASmallGraphFitsNoFurtherOutThanItsLabelsRead(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (x) => Math.round(x * 1000) / 1000;
out([r(P.fitFloor(819, "scope")), r(P.fitFloor(60, "neighbourhood")), r(P.fitFloor(150, "plan")), P.fitFloor(819, "plan"), P.fitFloor(20, "milestones") < 0]);`)
	assert.JSONEq(t, `[-0.29, -0.29, -0.29, null, true]`, got)
}

// TestPage_TheGraphDrawsByLabelSizeWithItsOwnZoomAndKeys is 1.17.1's
// readability and full-screen work at the page: each frame picks its level
// of detail from the label's size on screen, draws the whole id, smaller in
// more lines where two would not hold it, tints far nodes toward their band and drops their
// borders, takes its colours from tpPure.graphPalette, labels a band as a
// pill inside its corner, fits a small graph no further out than its
// labels read, fits again on entering or leaving full screen only when the
// camera sat at its fit, carries its own + − Fit and a ? for the keys, and
// in full screen puts the count at the toolbar's end. The count line no
// longer names layers and columns, nor the legend the renderer's time.
func TestPage_TheGraphDrawsByLabelSizeWithItsOwnZoomAndKeys(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		"G.detail = pure.detailLevel(P.geo.fs * scale);", "pure.fitId(out.label, geo.chars)", "const nodeText = (n) => n.text;", "getSize: (n) => P.geo.fs * n.fit,", `getAlignmentBaseline: "center", lineHeight: 1.25,`,
		"epic: { w: 150, h: 32, col: 172, row: 44, fs: 11, chars: 20 },",
		"mix(mark, bandFill(c, n.band), REGION_TINT)", "return G.detail === \"far\" ? 0 : 1;", "G.colors = pure.graphPalette(tok);",
		"background: true, getBackgroundColor: [...c.page, 235]", "pure.fitFloor(P.nodes.length, P.L.view, P.geo.fs)",
		"if (refit && G.prepared && G.deck && G.atFit) fit();", `$("gc-zin").addEventListener("click"`, `$("gc-nfs").textContent = $("gc-n").textContent;`,
		`crossFar: [24, 28, 0.6], cross: [40, 45, 0.75]`, `tree: [170, 150, 1.4]`,
	} {
		assert.Contains(t, script, s)
	}
	for _, s := range []string{`<div class="gzoom" role="group"`, `id="gc-zin"`, `id="gc-zout"`, `id="gc-zfit"`, `data-info="keys"`, `id="gc-nfs"`, ".gcanvas { overscroll-behavior-x: none; }"} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{"graph.loadedIn", "{layers} layers", "MIN_TEXT_PX"} {
		assert.NotContains(t, script, s)
	}
}

// TestPure_AnISOWeekReadsAsItsDates: the throughput figure names its week
// as the Monday-to-Sunday dates the locale writes, not as 2026-W48; a value
// that is no ISO week reads as itself.
func TestPure_AnISOWeekReadsAsItsDates(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.weekRange("2026-W48", "tr"), P.weekRange("2026-W48", "en"), P.weekRange("2026-W01", "en"), P.weekRange("soon", "en")]);`)
	var weeks []string
	require.NoError(t, json.Unmarshal([]byte(got), &weeks))
	assert.Contains(t, weeks[0], "23")
	assert.Contains(t, weeks[0], "29")
	assert.Contains(t, weeks[0], "Kas")
	assert.Contains(t, weeks[1], "Nov")
	assert.Contains(t, weeks[2], "Dec")
	assert.Contains(t, weeks[2], "Jan")
	assert.Equal(t, "soon", weeks[3])
}

// TestPage_TheAuditsPageItemsAreInPlace is the 1.17.1 audit's page items:
// the theme buttons carry an icon beside their word; the KPI's small text
// never wraps and leaves room for its (i); a milestone's drawer lists its
// epics and shows its gate's closure on the graph; every selection made
// outside the drawer is where Esc gives focus back; Review without a round
// is skipped, not ticked; an epic named on the Specs tab or a spec's header
// opens its drawer; and each operator command carries a Copy button. The
// audit's frozen grid columns under an open drawer were tried and left out
// (spec/1.17.1-measurements.md).
func TestPage_TheAuditsPageItemsAreInPlace(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{`<svg class="ti" aria-hidden="true"`, ".kpi .v small { white-space: nowrap; }", ".kpi .v { padding-right: 30px; }"} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{
		`data-closure="`, `t("drawer.members", { n: members.length })`,
		"app.showClosure = (ms) =>", "function noteOpener() {", `(p === "ground" || p === "review") && st === "done"`,
		"? epicLink(f.epic) + (f.epic_state", `esc(t("spec.epicLabel")) + " " + epicLink(d.epic)`, `if (state.tab !== "epics") {`,
		"const done = (id) => copyCmd(", "tpPure.weekRange(last.week, t.locale)",
	} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheEpicDrawerIsQuiet: the epic drawer's stepper is one line with
// each step's detail in its tooltip; its status names no dependency the
// list below already shows; a dependency row is a dot, an id and its why;
// a list past five rows folds the rest, and the graph history folds too,
// both staying open across redraws; an empty spec list is left out, and the
// description reads as prose.
func TestPage_TheEpicDrawerIsQuiet(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{".stepper.e3 .sd { display: none; }", ".stepper.e3 li + li::before {", ".drows .dwhy { font-size: var(--text-xs); line-height: 1.4; max-height: 2.8em; overflow: hidden; }",
		".ddesc { font: 400 var(--text-sm)/1.6 var(--font-body); color: var(--ink-2); background: none; border: 0;"} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{
		`'<li class="' + st + '" title="' + esc(sub[k]) + '"'`, `[t("drawer.f.status"), word(STAGE_LABEL, e.status)],`,
		"const [shown, rest] = tpPure.foldAt(list, 5);", `(dfolds.has(key) ? " open" : "")`, `document.addEventListener("toggle", (ev) => {`,
		`fold("history", "<summary>" + esc(t("drawer.historyShow")) + "</summary>" + epicHistoryHTML(id))`,
		`((e.files || []).length ? "<div><h4>" + esc(t("drawer.specs"))`, "style=\"background:var(' + tpPure.markToken(mk) + ')\"",
	} {
		assert.Contains(t, script, s)
	}
	for _, s := range []string{"drawer.noSpecs", `t("ms.waitsOn", { on: waits.join`} {
		assert.NotContains(t, script, s)
	}
}

// TestPage_TheFullScreenPillKeepsItsControlsWhole: beside an open drawer
// the full-screen toolbar keeps every control on one line and whole, and
// its count gives way first, as the 1440 × 900 screenshot with an epic
// selected showed it must.
func TestPage_TheFullScreenPillKeepsItsControlsWhole(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, "body.mapfs .gcard.fs .toolbar > *, body.mapfs .gcard.fs .toolbar .seg button { flex: none; white-space: nowrap; }")
	assert.Contains(t, page, "@container (max-width: 1240px) { body.mapfs .gcard.fs .gcountfs { display: none; } }")
}

// TestPage_ADoneEpicHasNoCurrentStep: a done epic's stepper marks no step
// current, and a step it never reached, such as binding a spec, as skipped.
func TestPage_ADoneEpicHasNoCurrentStep(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"const cur = reached.done ? -1 : ESTEPS.findIndex(([k]) => !reached[k]);", `if (st === "todo" && reached.done) st = "skip";`} {
		assert.Contains(t, script, s)
	}
}

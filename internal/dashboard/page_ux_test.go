package dashboard_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runPure runs prog under node after the page's pure.js, with its functions
// bound to P, and returns what prog writes. Without node it skips locally
// and fails under CI, where it must run.
func runPure(t *testing.T, prog string) string {
	t.Helper()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/pure.js")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	node, err := exec.LookPath("node")
	if err != nil && os.Getenv("CI") != "" {
		t.Fatal("node is not on PATH under CI (CI is set, as GitHub Actions sets CI=true): the page's pure functions must run there")
	}
	if err != nil {
		t.Skip("node is not on PATH")
	}
	src := string(body) + "\nconst P = module.exports;\nconst out = (v) => process.stdout.write(JSON.stringify(v));\n" + prog
	got, err := exec.CommandContext(t.Context(), node, "-e", src).CombinedOutput()
	require.NoError(t, err, string(got))
	return string(got)
}

// TestPage_LoadsThePureFunctionsBeforeTheScript: the page's pure functions
// live in pure.js, which the page loads before app.js so the script finds
// them, and which node loads alone in the tests.
func TestPage_LoadsThePureFunctionsBeforeTheScript(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	pure := strings.Index(page, `src="pure.js"`)
	app := strings.Index(page, `src="app.js"`)
	require.GreaterOrEqual(t, pure, 0, "the page loads pure.js")
	assert.Less(t, pure, app, "pure.js loads before app.js")
}

// TestPure_TheFirstViewFitsTheWidthAtAReadableScale is the map's first view
// (spec/backlog/dashboard-ux.md §2): the drawing's width fits the box, never
// below the readable scale, where the view starts at the top left and the
// rest is panned to, and never above the largest scale, so a small drawing is
// not blown up.
func TestPure_TheFirstViewFitsTheWidthAtAReadableScale(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.initialView(1184, 3000, 1400, 660), P.initialView(1184, 3000, 358, 500), P.initialView(200, 100, 1400, 660)]);`)
	assert.JSONEq(t, `[
		{"x": -8, "y": -8, "w": 1200, "h": 565.7142857142857},
		{"x": -8, "y": -8, "w": 358, "h": 500},
		{"x": -8, "y": -8, "w": 933.3333333333334, "h": 440}
	]`, got)
}

// TestPage_TheMapGoesFullScreen is the map's full-screen control
// (spec/backlog/dashboard-ux.md §2): a toolbar button asks the Fullscreen API
// for the page and marks the map to fill it, leaving full screen (Esc, which
// the browser handles) unmarks it, and while full screen the plain wheel
// zooms, as Ctrl or ⌘ with the wheel does outside it.
func TestPage_TheMapGoesFullScreen(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="f-full"`)
	for _, s := range []string{"requestFullscreen(", "exitFullscreen(", `"fullscreenchange"`, "document.fullscreenElement", `"mapfs"`} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, "body.mapfs")
}

// TestPure_APinchZoomsAboutTheFingersAndFollowsThem is touch on the map
// (spec/backlog/dashboard-ux.md §2): two fingers moving apart zoom in by the
// ratio of their spans, and the drawing point under their first midpoint
// stays under their current one, so two fingers moving together pan.
func TestPure_APinchZoomsAboutTheFingersAndFollowsThem(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (v) => Object.fromEntries(Object.entries(v).map(([k, x]) => [k, Math.round(x * 100) / 100]));
const v0 = { x: 0, y: 0, w: 400, h: 200 };
out([
  r(P.pinchView(v0, { x: 100, y: 100 }, { x: 300, y: 100 }, { x: 50, y: 100 }, { x: 350, y: 100 }, 400, 200)),
  r(P.pinchView(v0, { x: 100, y: 100 }, { x: 300, y: 100 }, { x: 150, y: 150 }, { x: 350, y: 150 }, 400, 200)),
]);`)
	assert.JSONEq(t, `[{"x": 66.67, "y": 33.33, "w": 266.67, "h": 133.33}, {"x": -50, "y": -50, "w": 400, "h": 200}]`, got)
}

// TestPage_TheMapTakesTouch: a touch pans the map with one finger and pinches
// it with two, so the map no longer leaves touch to the page.
func TestPage_TheMapTakesTouch(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.NotContains(t, script, `ev.pointerType === "touch") return`)
	assert.Contains(t, script, "tpPure.pinchView(")
	assert.Contains(t, page, "touch-action: none")
}

// TestPure_CentringKeepsTheZoomUnlessItIsUnreadable: taking the view to an
// epic centres it at the current zoom, zoomed in to the readable scale when
// the view was further out.
func TestPure_CentringKeepsTheZoomUnlessItIsUnreadable(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.centreView({ x: 0, y: 0, w: 400, h: 200 }, { x: 1000, y: 500 }, 400), P.centreView({ x: 0, y: 0, w: 1200, h: 600 }, { x: 1000, y: 500 }, 400)]);`)
	assert.JSONEq(t, `[{"x": 800, "y": 400, "w": 400, "h": 200}, {"x": 800, "y": 400, "w": 400, "h": 200}]`, got)
}

// TestPage_SearchAndListsTakeTheMapToTheEpic is the map's selection rule
// (spec/backlog/dashboard-ux.md §2): the search centres its first match as
// it is typed and selects it on Enter, and selecting an epic from any list
// centres it on the map.
func TestPage_SearchAndListsTakeTheMapToTheEpic(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "tpPure.centreView(")
	assert.Contains(t, script, `$("map-q").addEventListener("keydown"`)
	assert.Contains(t, script, "selectEpic(a.dataset.epic, true)")
}

// TestPage_TheDrawerNarrowsThePageInsteadOfCoveringIt is the drawer rule
// (spec/backlog/dashboard-ux.md §2): an open drawer takes its width from the
// page, beside it, and on a phone its height from below, so the map and a
// full-screen card stay whole beside it.
func TestPage_TheDrawerNarrowsThePageInsteadOfCoveringIt(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	for _, rule := range []string{"body.dopen { padding-right: var(--drawer-w); }", "body.mapfs.dopen .card.fs { right: var(--drawer-w); }",
		"body.dopen { padding-right: 0; padding-bottom: var(--drawer-h); }", "body.mapfs.dopen .card.fs { right: 0; bottom: var(--drawer-h); }"} {
		assert.Contains(t, page, rule)
	}
}

// TestPure_ArrowKeysMoveToTheNearestEpicThatWay is the map's roving focus
// (spec/backlog/dashboard-ux.md §2): an arrow moves to the nearest epic in
// its direction, a step across counting thrice a step along, and stays put
// when nothing lies that way.
func TestPure_ArrowKeysMoveToTheNearestEpicThatWay(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const pos = { a: { x: 0, y: 0 }, b: { x: 0, y: 10 }, c: { x: 100, y: 0 }, d: { x: 100, y: 30 } };
out([P.roveNext(pos, "a", "ArrowDown"), P.roveNext(pos, "a", "ArrowRight"), P.roveNext(pos, "a", "ArrowUp"), P.roveNext(pos, "a", "ArrowLeft"),
  P.roveNext(pos, "b", "ArrowRight"), P.roveNext(pos, "d", "ArrowLeft"), P.roveNext(pos, "a", "Tab")]);`)
	assert.JSONEq(t, `["b", "c", "a", "a", "c", "b", "a"]`, got)
}

// TestPage_MapEpicsTakeFocusAndEnterSelects: one epic of the map is in the
// tab order at a time, the arrows move it, and Enter or Space selects it.
func TestPage_MapEpicsTakeFocusAndEnterSelects(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.roveNext(", `role="button" tabindex="`, `ev.key === "Enter" || ev.key === " "`} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheDrawerTakesFocusAndGivesItBack is the drawer's focus rule
// (spec/backlog/dashboard-ux.md §2): opening it moves focus to its close
// button, and closing it returns focus to what held it before, found again
// by its epic when a redraw replaced that element.
func TestPage_TheDrawerTakesFocusAndGivesItBack(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{`$("dclose").focus(`, "drawerReturn", "returnFocus("} {
		assert.Contains(t, script, s)
	}
}

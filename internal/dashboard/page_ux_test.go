package dashboard_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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

// TestPure_WaitingEpicsGroupUnderEachDirectBlocker is Waiting epics
// (spec/backlog/dashboard-ux.md §2): the waiting epics of open milestones
// are grouped by what they wait on directly, an epic under each of its
// blockers and an any-of group as one blocker; each group carries its
// blocker's state (startable when tp epic ready lists it, claimed when it is
// under way, a decision when it is a condition, else waiting, an any-of
// group taking its readiest member's); groups are ordered by their size, ties
// in the order their blocker is first met; the waiting epics of milestones
// not yet open are set apart, and open counts the distinct grouped epics.
func TestPure_WaitingEpicsGroupUnderEachDirectBlocker(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const ep = (id, milestone, status, by, any, cond) => ({ id, milestone, status, blocked_by: by || [], blocked_by_any_of: any || [], condition: !!cond });
const epics = {};
[ep("a", "M", "planned"), ep("b", "M", "claimed"), ep("k", "M", "planned", [], [], true), ep("c", "M", "planned", ["b"]),
 ep("x1", "M", "planned", ["a", "b"]), ep("x2", "M", "planned", ["a", "k"]), ep("x3", "M", "planned", [], [["a", "b"]]),
 ep("y1", "N", "planned", ["a"])].forEach((e) => { epics[e.id] = e; });
out(P.waitingGroups(["c", "x1", "x2", "x3", "y1"], epics, ["a"], { M: true, N: false }));`)
	assert.JSONEq(t, `{
		"groups": [
			{"blocker": "b", "any_of": null, "state": "claimed", "epics": ["c", "x1"]},
			{"blocker": "a", "any_of": null, "state": "startable", "epics": ["x1", "x2"]},
			{"blocker": "k", "any_of": null, "state": "decision", "epics": ["x2"]},
			{"blocker": null, "any_of": ["a", "b"], "state": "startable", "epics": ["x3"]}
		],
		"closed": ["y1"],
		"open": 4
	}`, got)
}

// TestPage_WaitingEpicsSitBelowThePlanMapInGroups: the Waiting epics card
// follows the Plan map, draws the groups waitingGroups returns with the rest
// behind "show all", and names the deferrable members its total leaves out.
func TestPage_WaitingEpicsSitBelowThePlanMapInGroups(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	mapAt := strings.Index(page, `id="mapcard"`)
	waitAt := strings.Index(page, `id="waitcard"`)
	require.GreaterOrEqual(t, waitAt, 0, "the page has a Waiting epics card")
	assert.Greater(t, waitAt, mapAt, "Waiting epics follows the Plan map")
	for _, s := range []string{"tpPure.waitingGroups(", "data-wall", "deferrable gate member", "in milestones not yet open"} {
		assert.Contains(t, script, s)
	}
}

// TestPure_ALayoutBecomesPointsColumnByLayer: a layout from /api/layout
// becomes drawing points, one column per layer and one row per order, a
// short layer centred against the longest, each band spanning its layers'
// columns, and the drawing's size.
func TestPure_ALayoutBecomesPointsColumnByLayer(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(P.placeLayout({ layers: 2, bands: [{ milestone: "M", from_layer: 0, to_layer: 1 }],
  nodes: [{ id: "a", layer: 0, order: 0 }, { id: "b", layer: 0, order: 1 }, { id: "c", layer: 1, order: 0 }] },
  { colW: 100, rowH: 20, left: 10, top: 5, head: 0, inset: 10 }));`)
	assert.JSONEq(t, `{"pos": {"a": {"x": 20, "y": 15}, "b": {"x": 20, "y": 35}, "c": {"x": 120, "y": 25}},
		"bands": [{"name": "M", "x": 10, "w": 200}], "W": 220, "H": 50}`, got)
}

// TestPure_AScopedViewOpensWholeWhenItFits: a scoped view opens centred
// and whole when it fits at a readable scale, never blown up past the
// largest; one that does not fit opens at its top left.
func TestPure_AScopedViewOpensWholeWhenItFits(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (v) => Object.fromEntries(Object.entries(v).map(([k, x]) => [k, Math.round(x * 100) / 100]));
out([r(P.containView(200, 100, 400, 400)), r(P.containView(2000, 500, 400, 400))]);`)
	assert.JSONEq(t, `[{"x": -33.33, "y": -83.33, "w": 266.67, "h": 266.67}, {"x": -8, "y": -8, "w": 400, "h": 400}]`, got)
}

// TestPage_DrawsTheThreeScopedViewsFromTheLayoutEndpoint is the scoped
// views at the page (spec/backlog/dashboard-ux.md §2): the Dependencies card
// switches between the milestones, a gate's closure and an epic's
// neighbourhood, each fetched from /api/layout and placed from its layers and
// orders alone; it draws the bands, the critical path, any-of forks and the
// neighbourhood's centre, can go full screen, keeps the chain as a
// collapsible text view, and keeps its view, milestone and depth in the
// address.
func TestPage_DrawsTheThreeScopedViewsFromTheLayoutEndpoint(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{`id="gsvg"`, `data-gv="milestones"`, `data-gv="scope"`, `data-gv="neighbourhood"`, `id="chaintext"`, `data-fs="chaincard"`} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{`"/api/layout?"`, "tpPure.placeLayout(", "tpPure.containView", ".bands", ".critical", ".any_of", ".centre", ".inbound"} {
		assert.Contains(t, script, s)
	}
	m := regexp.MustCompile(`const URL_KEYS = (\[.*\]);`).FindStringSubmatch(script)
	require.NotNil(t, m)
	var urlKeys []string
	require.NoError(t, json.Unmarshal([]byte(m[1]), &urlKeys))
	assert.Subset(t, urlKeys, []string{"gv", "gm", "gd"})
}

// TestPage_TheLoopsFigureSplitsStaleFromNeedingARound: the review loops
// KPI shows the loops that need a round, the summary's open loops less its
// stale ones, and names the stale ones apart.
func TestPage_TheLoopsFigureSplitsStaleFromNeedingARound(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "f.open_loops - (f.stale_loops || 0)")
	assert.Contains(t, script, "review loops needing a round")
}

// TestPage_ListTitlesWrap: a list row's title wraps instead of being cut to
// a few letters and an ellipsis in a narrow card.
func TestPage_ListTitlesWrap(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, ".row .t { min-width: 0; overflow-wrap: anywhere; }")
}

// TestPage_AGatesThresholdAndPurposeReadInFull: a milestone's entry shows
// its gate's threshold wrapped, never cut to an ellipsis, and the gate's
// purpose from the graph when it has one.
func TestPage_AGatesThresholdAndPurposeReadInFull(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, ".ms .th, .ms .purpose { overflow-wrap: anywhere; }")
	assert.NotContains(t, page, ".ms .th { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }")
	assert.Contains(t, script, ".purpose")
}

// TestPure_DescribedEpicsAreCountedPerMilestone: an epic counts as
// described when its description holds more than whitespace, as tp treats
// one, and each milestone gets its described and all epics.
func TestPure_DescribedEpicsAreCountedPerMilestone(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(P.describedCounts([{ id: "a", milestone: "M", description: "## Purpose" }, { id: "b", milestone: "M", description: "  \n" },
  { id: "c", milestone: "M" }, { id: "d", milestone: "N", description: "x" }]));`)
	assert.JSONEq(t, `{"M": {"described": 1, "all": 3}, "N": {"described": 1, "all": 1}}`, got)
}

// TestPage_MarksDescribedEpicsAndCountsThemPerMilestone: a described epic
// carries a mark on the map and in the dependency views, and each
// milestone's entry says how many of its epics are described.
func TestPage_MarksDescribedEpicsAndCountsThemPerMilestone(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.describedCounts(", "descMark(", " described"} {
		assert.Contains(t, script, s)
	}
}

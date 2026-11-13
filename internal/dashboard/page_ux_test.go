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

// TestPage_AMissingPureScriptSaysSoInsteadOfABlankPage: when pure.js did
// not load, the script shows that on the page and stops, rather than
// failing on its first call and leaving the page blank.
func TestPage_AMissingPureScriptSaysSoInsteadOfABlankPage(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	guard := strings.Index(script, "if (!window.tpPure)")
	require.GreaterOrEqual(t, guard, 0)
	assert.Contains(t, script, "pure.js did not load")
	assert.Less(t, guard, strings.Index(script, "tpPure."), "the guard runs before any use of tpPure")
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
	for _, s := range []string{"requestFullscreen(", "exitFullscreen(", `"fullscreenchange"`, "document.fullscreenElement", `"mapfs"`, "tpPure.wheelZooms("} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, "body.mapfs")
}

// TestPage_BothMapsSayHowFullScreenZoomsAndLeaves: the Plan map's help and
// the dependency views' each say that in full screen the plain wheel zooms
// and Esc leaves.
func TestPage_BothMapsSayHowFullScreenZoomsAndLeaves(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Equal(t, 2, strings.Count(script, "in full screen the wheel alone zooms and Esc leaves"))
}

// TestPure_ThePlainWheelZoomsOnlyInFullScreen: Ctrl or ⌘ with the wheel
// zooms anywhere; the plain wheel zooms only while the card fills the
// screen, and scrolls the page otherwise.
func TestPure_ThePlainWheelZoomsOnlyInFullScreen(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.wheelZooms({ ctrlKey: true }, false), P.wheelZooms({ metaKey: true }, false), P.wheelZooms({}, false), P.wheelZooms({}, true)]);`)
	assert.JSONEq(t, `[true, true, false, true]`, got)
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

// TestPure_APointerPansPinchesOrIsIgnored: a mouse's main button and one
// finger pan, a second finger pinches, and any other mouse button or a
// third finger is ignored.
func TestPure_APointerPansPinchesOrIsIgnored(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.pointerAction(0, { pointerType: "mouse", button: 0 }), P.pointerAction(0, { pointerType: "mouse", button: 2 }),
  P.pointerAction(0, { pointerType: "pen", button: 0 }), P.pointerAction(1, { pointerType: "touch", button: 0 }),
  P.pointerAction(2, { pointerType: "touch", button: -1 }), P.pointerAction(3, { pointerType: "touch", button: -1 }),
]);`)
	assert.JSONEq(t, `["pan", "none", "pan", "pan", "pinch", "none"]`, got)
}

// TestPage_TheMapTakesTouch: a touch pans the map with one finger and pinches
// it with two, so the map no longer leaves touch to the page.
func TestPage_TheMapTakesTouch(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.NotContains(t, script, `ev.pointerType === "touch") return`)
	assert.Contains(t, script, "tpPure.pinchView(")
	assert.Contains(t, script, "tpPure.pointerAction(")
	assert.Contains(t, page, "touch-action: none")
}

// TestPage_TheMapsOneStepFilterReadsDirectOnly: the chip that narrows the
// map to the selected epic's direct edges is named Direct only, on the
// page and in the help that names it.
func TestPage_TheMapsOneStepFilterReadsDirectOnly(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `>Direct only</button>`)
	assert.NotContains(t, page, `>Neighbours<`)
	assert.NotContains(t, script, "Neighbours")
	assert.Equal(t, 2, strings.Count(script, "Direct only"), "the map's help and its legend name the chip")
}

// TestPure_CentringKeepsTheZoomUnlessItIsUnreadable: taking the view to an
// epic centres it at the current zoom, zoomed in to the readable scale when
// the view was further out.
func TestPure_CentringKeepsTheZoomUnlessItIsUnreadable(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.centreView({ x: 0, y: 0, w: 400, h: 200 }, { x: 1000, y: 500 }, 400), P.centreView({ x: 0, y: 0, w: 1200, h: 600 }, { x: 1000, y: 500 }, 400)]);`)
	assert.JSONEq(t, `[{"x": 800, "y": 400, "w": 400, "h": 200}, {"x": 800, "y": 400, "w": 400, "h": 200}]`, got)
}

// TestPure_ReCentringANeighbourhoodShowsItWhole: re-centring the
// neighbourhood view on an epic fits the new layout and puts the epic at the
// view's centre, zoomed out as far as that takes to keep every node in the
// box, never above the largest scale; 1.13.0 kept the zoom, so at depth 2 the
// outer nodes sat outside the box until Fit (1.14.1).
func TestPure_ReCentringANeighbourhoodShowsItWhole(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (v) => Object.fromEntries(Object.entries(v).map(([k, x]) => [k, Math.round(x * 100) / 100]));
out([r(P.containAt(1240, 110, 966, 576, { x: 600, y: 50 })), r(P.containAt(1240, 110, 966, 576, { x: 1000, y: 50 })), r(P.containAt(100, 50, 400, 400, { x: 50, y: 25 }))]);`)
	assert.JSONEq(t, `[{"x": -48, "y": -336.39, "w": 1296, "h": 772.77}, {"x": -8, "y": -551.04, "w": 2016, "h": 1202.09},
		{"x": -83.33, "y": -108.33, "w": 266.67, "h": 266.67}]`, got)
}

// TestPure_FitAtViewHoldsTheLayoutCentredOnTheEpic: the view re-centring
// takes for a drawing of size in a box of rect: containAt's, or none while
// the box has no size or there is no point to centre (1.14.1 review).
func TestPure_FitAtViewHoldsTheLayoutCentredOnTheEpic(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (v) => v && Object.fromEntries(Object.entries(v).map(([k, x]) => [k, Math.round(x * 100) / 100]));
const size = { W: 1240, H: 110 };
out([r(P.fitAtView(size, { width: 966, height: 576 }, { x: 600, y: 50 })), P.fitAtView(size, { width: 0, height: 576 }, { x: 600, y: 50 }),
  P.fitAtView(size, { width: 966, height: 0 }, { x: 600, y: 50 }), P.fitAtView(size, { width: 966, height: 576 }, null)]);`)
	assert.JSONEq(t, `[{"x": -48, "y": -336.39, "w": 1296, "h": 772.77}, null, null, null]`, got)
}

// TestPage_ReCentringFitsTheNewLayout: a neighbourhood drawn for a new key
// takes the view containAt gives for its centre epic, not the old zoom.
func TestPage_ReCentringFitsTheNewLayout(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "tpPure.fitAtView(size, r, p)")
	assert.Contains(t, script, "gport.fitAt(geo.pos[L.epic])")
	assert.NotContains(t, script, "gport.centre(geo.pos[L.epic])")
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
	assert.Contains(t, script, "if (centre && mapView.selected && mapView.geo) mapPort.centre(", "selectEpic centres the map when asked to")
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

// TestPure_ExactlyOneEpicIsATabStop: of a view's epics only the focused
// one takes tabindex 0, the first when the focus is on none of them.
func TestPure_ExactlyOneEpicIsATabStop(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.tabStops(["a", "b", "c"], "b"), P.tabStops(["a", "b", "c"], "z"), P.tabStops(["a", "b"], null), P.tabStops([], "a")]);`)
	assert.JSONEq(t, `[[-1, 0, -1], [0, -1, -1], [0, -1], []]`, got)
}

// TestPage_MapEpicsTakeFocusAndEnterSelects: one epic of the map is in the
// tab order at a time, the arrows move it, and Enter or Space selects it.
func TestPage_MapEpicsTakeFocusAndEnterSelects(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Equal(t, 2, strings.Count(script, "tpPure.tabStops("), "the map and the dependency view each take their tab stops from tabStops")
	for _, s := range []string{"tpPure.roveNext(", `ev.key === "Enter" || ev.key === " "`} {
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
	for _, s := range []string{`$("dclose").focus(`, "tpPure.focusAfterClose(", "if (!open && drawerReturn) returnFocus();", "data-wg="} {
		assert.Contains(t, script, s)
	}
}

// TestPure_ClosingTheDrawerGivesFocusBackToWhatOpenedIt: focus goes back to
// the element that opened the drawer while it is still on the page, else to
// the one a redraw put in its place, found by the key saved at opening (a
// map epic, a dependency-view node, a waiting group's link by its group, a
// list link by its box); nothing when none is there.
func TestPure_ClosingTheDrawerGivesFocusBackToWhatOpenedIt(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.focusAfterClose({ connected: true, keys: ["map:a"] }, []),
  P.focusAfterClose({ connected: false, keys: ["graph:a"] }, ["map:a", "graph:a"]),
  P.focusAfterClose({ connected: false, keys: ["wait:2:x"] }, ["wait:1:x", "wait:2:x", "link:now-ready:x"]),
  P.focusAfterClose({ connected: false, keys: ["map:q"] }, ["map:a"]),
  P.focusAfterClose(null, ["map:a"]),
]);`)
	assert.JSONEq(t, `["same", "graph:a", "wait:2:x", null, null]`, got)
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

// TestPure_WaitingGroupsSitInSectionsByTheBlockersMilestone: the blocker
// groups fall into one section per blocker's milestone, in the milestone
// order the graph file gives, a group keeping its rank by direct waiters
// inside its section; an any-of group sits under its earliest member's
// milestone; each section counts its groups and the distinct epics waiting
// in it; a blocker on the critical path is marked, an any-of group when a
// member is, and the mark does not reorder anything (1.14.1).
func TestPure_WaitingGroupsSitInSectionsByTheBlockersMilestone(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const epics = { t1: { milestone: "T" }, a1: { milestone: "A" }, a2: { milestone: "A" }, b1: { milestone: "B" } };
const groups = [
  { blocker: "t1", any_of: null, epics: ["x1", "x2", "x3"] },
  { blocker: "a1", any_of: null, epics: ["x1", "x4"] },
  { blocker: null, any_of: ["b1", "a2"], epics: ["x5"] },
  { blocker: "b1", any_of: null, epics: ["x6"] },
  { blocker: "a2", any_of: null, epics: ["x4"] },
];
out(P.waitingSections(groups, epics, ["A", "B", "T"], ["a2", "t1"]).map((s) => ({ m: s.milestone, n: s.epics, g: s.groups.map((g) => (g.blocker || g.any_of.join("|")) + (g.critical ? "*" : "")) })));`)
	assert.JSONEq(t, `[
		{"m": "A", "n": 3, "g": ["a1", "b1|a2*", "a2*"]},
		{"m": "B", "n": 1, "g": ["b1"]},
		{"m": "T", "n": 3, "g": ["t1*"]}
	]`, got)
}

// TestPure_ASectionOutsideTheMilestoneOrderSortsLast: a blocker whose
// milestone the graph file's order does not hold, or a blocker the export
// does not hold, puts its section after every ordered one (1.14.1 review).
func TestPure_ASectionOutsideTheMilestoneOrderSortsLast(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const epics = { z1: { milestone: "Z" }, a1: { milestone: "A" }, b1: { milestone: "B" } };
const groups = [{ blocker: "z1", any_of: null, epics: ["x1"] }, { blocker: "gone", any_of: null, epics: ["x2"] }, { blocker: "b1", any_of: null, epics: ["x3"] }, { blocker: "a1", any_of: null, epics: ["x4"] }];
out(P.waitingSections(groups, epics, ["A", "B"], []).map((s) => s.milestone));`)
	assert.JSONEq(t, `["A", "B", "Z", ""]`, got)
}

// TestPure_AWaitingSectionKeepsItsOwnFoldAndShowAll: folding section A
// leaves section B as it was and holds across redraws; B's Show all opens
// B alone; a section with no milestone names it "no milestone" in its
// header and its Show all alike; each button's aria-expanded comes from
// the same state (1.14.1 review).
func TestPure_AWaitingSectionKeepsItsOwnFoldAndShowAll(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const sec = (m, n) => ({ milestone: m, epics: n, groups: Array.from({ length: n }, (_, i) => ({ blocker: m + i })) });
const secs = [sec("A", 5), sec("B", 4), sec("", 4)];
const draw = (v) => secs.map((s) => P.waitSection(s, v, 3)).map((w) => ({ key: w.key, title: w.title, expanded: w.expanded, shown: w.groups.length, more: w.more }));
let v = { all: new Set(), folded: new Set() };
const first = draw(v);
v = P.flipWait(v, "folded", "A");
const folded = draw(v);
const again = draw(v);
v = P.flipWait(v, "all", "B");
out({ first, folded, same: JSON.stringify(folded) === JSON.stringify(again), opened: draw(v) });`)
	assert.JSONEq(t, `{
		"first": [
			{"key": "A", "title": "Blockers in A", "expanded": true, "shown": 3, "more": {"expanded": false, "text": "Show all 5 blockers in A"}},
			{"key": "B", "title": "Blockers in B", "expanded": true, "shown": 3, "more": {"expanded": false, "text": "Show all 4 blockers in B"}},
			{"key": "", "title": "Blockers in no milestone", "expanded": true, "shown": 3, "more": {"expanded": false, "text": "Show all 4 blockers in no milestone"}}
		],
		"folded": [
			{"key": "A", "title": "Blockers in A", "expanded": false, "shown": 0, "more": null},
			{"key": "B", "title": "Blockers in B", "expanded": true, "shown": 3, "more": {"expanded": false, "text": "Show all 4 blockers in B"}},
			{"key": "", "title": "Blockers in no milestone", "expanded": true, "shown": 3, "more": {"expanded": false, "text": "Show all 4 blockers in no milestone"}}
		],
		"same": true,
		"opened": [
			{"key": "A", "title": "Blockers in A", "expanded": false, "shown": 0, "more": null},
			{"key": "B", "title": "Blockers in B", "expanded": true, "shown": 4, "more": {"expanded": true, "text": "Show the largest 3"}},
			{"key": "", "title": "Blockers in no milestone", "expanded": true, "shown": 3, "more": {"expanded": false, "text": "Show all 4 blockers in no milestone"}}
		]
	}`, got)
}

// TestPage_AWaitingSectionDrawsWhatWaitSectionDecides: the page draws a
// section from waitSection alone, each button's aria-expanded included,
// and flips its state through flipWait.
func TestPage_AWaitingSectionDrawsWhatWaitSectionDecides(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.waitSection(s, waitView, WAIT_SHOWN)", `aria-expanded="' + w.expanded + '"`, `aria-expanded="' + w.more.expanded + '"`, "waitView = tpPure.flipWait(waitView, "} {
		assert.Contains(t, script, s)
	}
}

// TestPage_WaitingEpicsReadInSectionsThatFoldAndShowAll: Waiting epics
// draws a section per blocker milestone, its header naming the milestone
// and its counts, each folding on its own and with its own "show all", and
// a critical-path badge on a blocker on the critical path.
func TestPage_WaitingEpicsReadInSectionsThatFoldAndShowAll(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.waitingSections(", "data-wsec=", "data-wall=", "tpPure.flipWait(", "on the critical path"} {
		assert.Contains(t, script, s)
	}
}

// TestPure_EveryStateWordReadsTheOneStatusField: the map's marks and the
// Waiting pills read an epic's stage from the status field the export
// serves and from nothing else, so a specced epic (a bound spec, no claim)
// is specced everywhere, never "claimed" in a pill or "claimed or under way"
// on the map, and the Lifecycle count of each stage, the work epics with
// that status, equals the marks of that stage; a planned epic's mark says
// what holds it (1.14.1). An epic past planned that is also blocked or
// held keeps both facts: its stage as the mark, the hold as an overlay and
// in its words, "claimed, blocked", as the KPI and Waiting count it blocked
// (1.14.1 review).
func TestPure_EveryStateWordReadsTheOneStatusField(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const ep = (id, status, x) => Object.assign({ id, milestone: "M", status, blocked_by: [], blocked_by_any_of: [] }, x || {});
const work = [ep("a", "planned"), ep("b", "planned", { blocked: true }), ep("g", "planned", { held: true, blocked: true }), ep("c", "claimed", { blocked: true }),
  ep("d", "specced"), ep("h", "specced", { milestone: "N" }), ep("i", "specced", { held: true, blocked: true }), ep("e", "in_progress", { blocked: true }), ep("f", "done")];
const closed = { N: true };
const marks = {};
const holds = {};
const words = {};
const LABEL = { claimed: "claimed", specced: "specced", in_progress: "in progress", blocked: "blocked", held: "held by a condition", ready: "can start", done: "done", waiting: "milestone closed" };
work.forEach((e) => {
  const m = P.markState(e, !!closed[e.milestone]);
  marks[m.mark] = (marks[m.mark] || 0) + 1;
  if (m.hold) holds[e.id] = m.hold;
  words[e.id] = P.markText(m, LABEL);
});
const band = {};
work.forEach((e) => { band[e.status] = (band[e.status] || 0) + 1; });
const epics = {};
work.concat([ep("x", "planned", { blocked: true, blocked_by: ["d", "e", "c"] }), ep("y", "planned", { blocked: true, blocked_by_any_of: [["d", "c"]] })]).forEach((e) => { epics[e.id] = e; });
const w = P.waitingGroups(["x", "y"], epics, [], { M: true });
out({ marks, band, holds, words, pills: w.groups.map((g) => (g.blocker || g.any_of.join("|")) + ":" + g.state) });`)
	assert.JSONEq(t, `{
		"marks": {"ready": 1, "blocked": 1, "held": 1, "claimed": 1, "specced": 3, "in_progress": 1, "done": 1},
		"band": {"planned": 3, "claimed": 1, "specced": 3, "in_progress": 1, "done": 1},
		"holds": {"c": "blocked", "i": "held", "e": "blocked"},
		"words": {"a": "can start", "b": "blocked", "g": "held by a condition", "c": "claimed, blocked", "d": "specced", "h": "specced",
			"i": "specced, held by a condition", "e": "in progress, blocked", "f": "done"},
		"pills": ["d:specced", "e:in_progress", "c:claimed", "d|c:specced"]
	}`, got)
}

// TestPage_MapNodesCarryTheirStatus: each map node carries the status the
// export serves, and the legend names the three stages after planned.
func TestPage_MapNodesCarryTheirStatus(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `data-status="' + esc(e.status)`)
	assert.Contains(t, script, "tpPure.markState(")
	assert.NotContains(t, script, "claimed or under way")
	assert.NotContains(t, script, "function markState(")
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

// TestPage_AWaitingGroupSaysItCountsOnlyThisList: a group's count reads
// as the epics of this list, those of open milestones, that wait on its
// blocker, not every epic that does.
func TestPage_AWaitingGroupSaysItCountsOnlyThisList(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `" on it here</span></div>"`)
	assert.Contains(t, script, `title="epics of open milestones that wait on it directly"`)
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

// TestPure_AMergedBandIsNamedForEveryMilestoneItHolds: a band tp reports
// with others sharing its span, a cycle between milestones, is drawn once
// and labelled with each of its milestones.
func TestPure_AMergedBandIsNamedForEveryMilestoneItHolds(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(P.placeLayout({ layers: 2, bands: [{ milestone: "M1", with: ["M2"], from_layer: 0, to_layer: 1 }],
  nodes: [{ id: "a", layer: 0, order: 0 }, { id: "b", layer: 1, order: 0 }] },
  { colW: 100, rowH: 20, left: 10, top: 5, head: 0, inset: 10 }).bands);`)
	assert.JSONEq(t, `[{"name": "M1 + M2", "x": 10, "w": 200}]`, got)
}

// TestPure_ADependencyLabelSitsOnAnOpaqueBox: an epic's id label in the
// dependency views sits on a box as wide as its characters, so an edge
// passing behind it never runs through the text, and the epic's edges leave
// from the box's right end rather than from inside a long label (1.14.1).
func TestPure_ADependencyLabelSitsOnAnOpaqueBox(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.labelBox(100, 50, 10), P.labelBox(0, 0, 0)]);`)
	assert.JSONEq(t, `[{"x": 98, "y": 40, "w": 71, "h": 14}, {"x": -2, "y": -10, "w": 4, "h": 14}]`, got)
}

// TestPage_DependencyEdgesRunBehindOpaqueLabels: the dependency views draw
// their edges first and each epic's label on its box after them, the box
// filled with the card's surface.
func TestPage_DependencyEdgesRunBehindOpaqueLabels(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `'<rect class="lbg"`)
	assert.Contains(t, page, ".map .lbg { fill: var(--surface); }")
	edges := strings.Index(script, `"<g>" + gedgesHTML(`)
	nodes := strings.Index(script, "L.nodes.map((n, i) => gnodeHTML(")
	require.GreaterOrEqual(t, edges, 0)
	assert.Less(t, edges, nodes, "edges are drawn before the nodes, so the nodes cover them")
}

// TestPage_AMilestoneSaysItsInboundCountIsEpicEdges: the Milestones view
// draws opens-after lines between milestones, while a milestone's inbound
// count counts dependency edges between epics, so the number says "epic
// edges in", never a bare "edges in" that reads as the lines drawn (1.14.1).
func TestPage_AMilestoneSaysItsInboundCountIsEpicEdges(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `plural(n.inbound, "epic edge", "epic edges") + " in"`)
	assert.Contains(t, script, `n.inbound + " epic edges in"`)
	assert.NotContains(t, script, `plural(n.inbound, "edge", "edges")`)
}

// TestPure_TheDependencyViewDrawsOnlyTheDataOfItsKey: a view draws only
// the layout fetched for the query it shows; until that layout arrives it
// reads as loading, even with another query's layout still held, so a
// click or a milestone change never draws, fits or centres stale data. An
// error shows for the query it came from, and no query shows nothing.
func TestPure_TheDependencyViewDrawsOnlyTheDataOfItsKey(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const L = { view: "neighbourhood" };
out([
  P.layoutReady({ key: null, dataKey: null, data: null, error: null }),
  P.layoutReady({ key: "a", dataKey: null, data: null, error: null }),
  P.layoutReady({ key: "b", dataKey: "a", data: L, error: null }),
  P.layoutReady({ key: "b", dataKey: "b", data: L, error: null }),
  P.layoutReady({ key: "b", dataKey: "a", data: L, error: { error: "x" } }),
]);`)
	assert.JSONEq(t, `["none", "loading", "loading", "draw", "error"]`, got)
}

// TestPage_TheDependencyViewRecordsTheKeyItsDataCameFor: the page keeps the
// key each layout was fetched for, clears an error when a new key starts,
// and asks layoutReady before drawing.
func TestPage_TheDependencyViewRecordsTheKeyItsDataCameFor(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.layoutReady(gview)", "dataKey: key", "Object.assign(gview, { key, error: null })"} {
		assert.Contains(t, script, s)
	}
}

// TestPure_AScopedViewOpensWhole: a scoped view opens, and Fit shows it,
// centred and whole, never blown up past the largest scale; one too large
// for the readable scale is zoomed out until it fits, so no band of a large
// gate closure and no milestone of a long chain is cut off (1.14.1).
func TestPure_AScopedViewOpensWhole(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (v) => Object.fromEntries(Object.entries(v).map(([k, x]) => [k, Math.round(x * 100) / 100]));
out([r(P.containView(200, 100, 400, 400)), r(P.containView(2000, 500, 400, 400)), r(P.containView(3592, 316, 1406, 576))]);`)
	assert.JSONEq(t, `[{"x": -33.33, "y": -83.33, "w": 266.67, "h": 266.67}, {"x": -8, "y": -758, "w": 2016, "h": 2016},
		{"x": -8, "y": -581.05, "w": 3608, "h": 1478.1}]`, got)
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

// TestPage_TheDependencyViewsHelpIsDefinedOnceAndNamesTheCentre: the
// views' help is one INFO.chain, which says where the neighbourhood's
// centre sits and its dependencies and dependents lie.
func TestPage_TheDependencyViewsHelpIsDefinedOnceAndNamesTheCentre(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Equal(t, 1, strings.Count(script, "INFO.chain ="))
	assert.Contains(t, script, "highlighted, its dependencies left, its dependents right")
	assert.NotContains(t, script, "the selected epic in the middle")
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

// TestPage_TheKPIRowReadsEvenlyAtDesktopWidth: the nine KPI cards are an
// auto-fit grid whose cards may narrow to 140 px, so at 1440 px (a 1408 px
// row, 8 px gaps: 149 px each) all nine share one row instead of eight and
// a ninth wrapping alone, as they did at a 160 px minimum (1.14.1).
func TestPage_TheKPIRowReadsEvenlyAtDesktopWidth(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, ".kpis { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 140px), 1fr));")
	assert.NotContains(t, page, "minmax(160px, 1fr)")
}

// TestPure_TheKPIRowsBalance: the KPI row takes the fewest rows its width
// allows for cards of at least minCard px, and the fewest columns giving
// that many rows, so nine cards read 9, 5 + 4 or 3 + 3 + 3, never 8 + 1 or
// 7 + 2; a row with no width (a hidden tab) or no cards has no answer yet
// (1.15.1).
func TestPure_TheKPIRowsBalance(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([1408, 1300, 1168, 968, 608, 358, 100, 0].map((w) => P.balancedColumns(9, w, 140, 8)).concat([P.balancedColumns(4, 1408, 140, 8), P.balancedColumns(0, 1408, 140, 8)]));`)
	assert.JSONEq(t, `[9, 5, 5, 5, 3, 2, 1, null, 4, null]`, got)
}

// TestPure_EveryRowSpansTheFullWidth: n cards in at most cols per row
// spread over the fewest rows as evenly as they divide, the fuller rows
// first, and each card spans its share of a grid as many columns wide as
// every row length divides, so a shorter row stretches its cards to the
// full width: 5 + 4 is twenty columns, four each on the first row and five
// on the second; a lone last card spans the row (1.15.1).
func TestPure_EveryRowSpansTheFullWidth(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const g = (n, c) => { const r = P.cardSpans(n, c); return r && r.columns + ":" + r.spans.join(""); };
out([g(9, 5), g(9, 9), g(9, 3), g(9, 2), g(14, 7), g(14, 5), g(5, 2), g(7, 3), g(0, 3), g(9, null)]);`)
	assert.JSONEq(t, `["20:444445555", "9:111111111", "3:111111111", "2:111111112", "7:11111111111111", "20:44444444445555", "2:11112", "6:2223333", null, null]`, got)
}

// TestPage_TheKPIRowFitsItsColumnsOnRenderAndResize: the page sets the
// KPI row's grid and each card's span from tpPure.balancedColumns and
// tpPure.cardSpans when it draws the row and whenever the row changes
// size, and the CSS reads the grid (1.15.1).
func TestPage_TheKPIRowFitsItsColumnsOnRenderAndResize(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{"tpPure.balancedColumns(", "tpPure.cardSpans(", `["#kpis", 140]`, `rowsSeen.observe($(id))`} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, script, "].join(\"\");\n    fitAllRows();", "the row is fitted as soon as it is drawn")
	assert.Contains(t, page, ".kpis[data-cols], .pcells[data-cols], .mscells[data-cols] { grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); }")
	assert.NotContains(t, page, ".lone")
}

// TestPure_TheEpicTotalsNameTheConditions: the Epics tab counts every epic
// while the share and the Lifecycle count work epics, so the totals read as
// one sentence that names the condition epics between them (1.14.1).
func TestPure_TheEpicTotalsNameTheConditions(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.epicTotals([{ id: "a" }, { id: "b" }, { id: "k", condition: true }]), P.epicTotals([{ id: "a" }]), P.epicTotals([{ id: "k", condition: true }])]);`)
	const tail = "; the Lifecycle and the share count the work epics"
	assert.JSONEq(t, `["3 epics: 2 work epics and 1 condition epic`+tail+`", "1 epic: 1 work epic and 0 condition epics`+tail+`", "1 epic: 0 work epics and 1 condition epic`+tail+`"]`, got)
}

// TestPage_TheTotalsSentenceSitsWhereBothCountsAppear: the sentence titles
// the Epics tab's count and the share KPI, and reads under the Lifecycle.
func TestPage_TheTotalsSentenceSitsWhereBothCountsAppear(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="totals"`)
	for _, s := range []string{"tpPure.epicTotals(", `$("tab-epics").title = `, `$("totals").textContent = `, "kpi(\"share\""} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, script, `"share of work epics done", "", totals`)
	assert.Equal(t, 1, strings.Count(script, "tpPure.epicTotals("), "the tab, the share and the Lifecycle read the sentence from one call")
}

// TestPage_ListTitlesWrap: a list row's title wraps instead of being cut to
// a few letters and an ellipsis in a narrow card.
func TestPage_ListTitlesWrap(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, ".row .t { min-width: 0; overflow-wrap: anywhere; }")
}

// TestPure_AMilestoneCardReadsOneLine: a milestone card's one line is its
// gate's purpose when it has one, since it says what the milestone is for,
// else its threshold as the card words it (1.15.1).
func TestPure_AMilestoneCardReadsOneLine(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.msLine("  A2 proves its slice  ", "threshold: p95"), P.msLine(" \n", "threshold: p95"), P.msLine(undefined, "no threshold")]);`)
	assert.JSONEq(t, `[{"kind": "purpose", "text": "A2 proves its slice"}, {"kind": "threshold", "text": "threshold: p95"}, {"kind": "threshold", "text": "no threshold"}]`, got)
}

// TestPage_AMilestoneCardIsOneLineThatOpensItsDetail: a milestone card is
// a button holding one line cut to its width by an ellipsis, the whole
// text in its title, and the cards top-aligned; 1.14.1's three-line clamp
// and its Show all are gone (1.15.1).
func TestPage_AMilestoneCardIsOneLineThatOpensItsDetail(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, ".mscells { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: var(--space-xs); align-items: start; }")
	assert.Contains(t, page, ".ms .line { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }")
	for _, s := range []string{`'<button type="button" class="ms" data-ms="'`, "tpPure.msLine(", `' title="' + esc(line.text) + '">'`, "openMilestone(b.dataset.ms)"} {
		assert.Contains(t, script, s)
	}
	for _, s := range []string{"clamp", "measureCards", "data-msx", "Show all of "} {
		assert.NotContains(t, page+script, s)
	}
}

// TestPage_AMilestonesDetailReadsItsGateInFull: the drawer shows a
// milestone's gate state, its threshold and purpose in full, wrapped, and
// how many of its epics are described, under a header that names a
// milestone; an epic selected after it takes the drawer over (1.15.1).
func TestPage_AMilestonesDetailReadsItsGateInFull(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="dkind"`)
	assert.Contains(t, page, ".dbody dd { margin: 0; min-width: 0; overflow-wrap: anywhere; white-space: pre-line; }")
	for _, s := range []string{"function milestoneHTML(", `["Gate", `, `["Threshold", `, `["Purpose", `, `["Described", `, `$("dkind").textContent = msd ? "Milestone" : "Epic";`, "msView.detail = null;"} {
		assert.Contains(t, script, s)
	}
}

// TestPure_AToggleKeepsItsKeyAcrossARedraw: a milestone card, a Waiting
// section's header and a section's Show all each have a key read from
// their data attribute, and the key gives that attribute and value back,
// so after a redraw focus returns to the replacement button; anything else
// has no key (1.14.1 review; the card opens the drawer since 1.15.1).
func TestPure_AToggleKeepsItsKeyAcrossARedraw(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const keys = [{ ms: "A2" }, { wsec: "T1" }, { wall: "C1" }, { wall: "" }, { epic: "x" }, {}].map(P.toggleKey);
out({ keys, back: keys.filter(Boolean).map(P.toggleOf) });`)
	assert.JSONEq(t, `{"keys": ["ms:A2", "wsec:T1", "wall:C1", "wall:", null, null],
		"back": [{"attr": "data-ms", "value": "A2"}, {"attr": "data-wsec", "value": "T1"}, {"attr": "data-wall", "value": "C1"}, {"attr": "data-wall", "value": ""}]}`, got)
}

// TestPage_AToggleGivesFocusBackToItsReplacement: the toggles and the
// milestone cards are among the focusable elements focusKey names; each
// Waiting toggle's handler puts focus back on its replacement after the
// redraw, and a milestone card gets it back when the drawer it opened
// closes.
func TestPage_AToggleGivesFocusBackToItsReplacement(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `const FOCUSABLE = "#mapsvg .node, #gsvg .gnode, [data-epic], [data-ms], [data-wsec], [data-wall]";`)
	assert.Contains(t, script, "tpPure.toggleKey(")
	assert.Equal(t, 1, strings.Count(script, "refocus(key);"), "the Waiting toggles refocus; a milestone card is given focus back by the drawer")
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

// TestPure_AGateClosureCountsItsDescribedEpics: a gate closure's header
// says how many of the closure's epics are described, read from the
// layout's nodes, since the per-milestone count does not answer it for a
// closure spanning milestones (1.14.1).
func TestPure_AGateClosureCountsItsDescribedEpics(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.closureDescribed([{ id: "a" }, { id: "b" }, { id: "c" }], new Set(["a", "c", "z"])), P.closureDescribed([], new Set(["a"]))]);`)
	assert.JSONEq(t, `["2 of 3 in the closure described", "0 of 0 in the closure described"]`, got)
}

// TestPage_TheClosureHeaderSaysHowManyAreDescribed: the closure view's
// header carries closureDescribed's sentence.
func TestPage_TheClosureHeaderSaysHowManyAreDescribed(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "tpPure.closureDescribed(L.nodes, ctx.described)")
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

// TestPage_PlanReviewNamesASetSliceByItsName: a set slice's row leads with
// the set's name and size instead of its whole id list, which follows as the
// command that reviews it.
func TestPage_PlanReviewNamesASetSliceByItsName(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "function sliceTitle(")
	assert.Contains(t, script, `"set " + k.name`)
	assert.Contains(t, script, "esc(sliceTitle(s))")
}

// TestPage_PlanReviewStacksIntoCardsOnAPhone: on a narrow screen each slice
// row becomes a card, each cell labelled by its column, instead of a table
// scrolled sideways.
func TestPage_PlanReviewStacksIntoCardsOnAPhone(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, "#slices .loops tr { display: grid;")
	assert.Contains(t, page, "#slices .loops td[data-l]::before { content: attr(data-l)")
	assert.Contains(t, script, `<td data-l="State">`)
}

// TestPure_TheUpdateTimeReadsRelative: the header's update time reads as
// how long ago tp last pushed, in the largest whole unit.
func TestPure_TheUpdateTimeReadsRelative(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([0, 9999, 10000, 59000, 60000, 3599000, 3600000, 7300000, -5].map(P.relTime));`)
	assert.JSONEq(t, `["just now", "just now", "10 s ago", "59 s ago", "1 min ago", "59 min ago", "1 h ago", "2 h ago", "just now"]`, got)
}

// TestPage_TheHeaderSaysHowLongAgoTpPushed: the header's update time is
// relative, kept current by a timer, the clock time in its title.
func TestPage_TheHeaderSaysHowLongAgoTpPushed(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "tpPure.relTime(")
	assert.Contains(t, script, "setInterval(")
}

// TestPage_HasAFaviconFromTheEmbed: the page links an SVG favicon the
// binary serves, so the tab is told apart without a request elsewhere.
func TestPage_HasAFaviconFromTheEmbed(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, `<link rel="icon" href="favicon.svg" type="image/svg+xml">`)
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/favicon.svg")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "image/svg+xml")
}

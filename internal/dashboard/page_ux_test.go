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
// bound to P and T the translator over en.json, and returns what prog
// writes. Without node it skips locally and fails under CI, where it must
// run.
func runPure(t *testing.T, prog string) string {
	t.Helper()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/pure.js")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	en := serve(t, http.MethodGet, "127.0.0.1:7377", "/i18n/en.json")
	require.Equal(t, http.StatusOK, en.StatusCode)
	dict, err := io.ReadAll(en.Body)
	require.NoError(t, err)
	node, err := exec.LookPath("node")
	if err != nil && os.Getenv("CI") != "" {
		t.Fatal("node is not on PATH under CI (CI is set, as GitHub Actions sets CI=true): the page's pure functions must run there")
	}
	if err != nil {
		t.Skip("node is not on PATH")
	}
	src := string(body) + "\nconst P = module.exports;\nconst T = P.translator(" + string(dict) + ", \"en\");\nconst out = (v) => process.stdout.write(JSON.stringify(v));\n" + prog
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
	pure := strings.Index(page, `src="pure.js?v=`)
	app := strings.Index(page, `src="app.js?v=`)
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

// TestPage_TheGraphGoesFullScreen is the card's full-screen control
// (spec/backlog/dashboard-ux.md §2): a toolbar button asks the Fullscreen API
// for the page and marks the card to fill it, leaving full screen (Esc, which
// the browser handles) unmarks it, and while full screen the plain wheel
// zooms, as Ctrl or ⌘ with the wheel does outside it; the help says so.
func TestPage_TheGraphGoesFullScreen(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="gc-full" data-fs="graphcard"`)
	for _, s := range []string{"requestFullscreen(", "exitFullscreen(", `"fullscreenchange"`, "document.fullscreenElement", `"mapfs"`, "pure.wheelZooms("} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, "body.mapfs")
	assert.Equal(t, 1, strings.Count(script, "in full screen the wheel alone zooms and Esc leaves"))
}

// TestPure_ThePlainWheelZoomsOnlyInFullScreen: Ctrl or ⌘ with the wheel
// zooms anywhere; the plain wheel zooms only while the card fills the
// screen, and scrolls the page otherwise.
func TestPure_ThePlainWheelZoomsOnlyInFullScreen(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.wheelZooms({ ctrlKey: true }, false), P.wheelZooms({ metaKey: true }, false), P.wheelZooms({}, false), P.wheelZooms({}, true)]);`)
	assert.JSONEq(t, `[true, true, false, true]`, got)
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

// TestPage_TheKeyboardRunsThroughSearchListsAndTheCanvasBox is decision 4
// of spec/backlog/dashboard-graph.md at the page: the canvas has no
// focusable node, its box takes focus and the arrows pan it and + and −
// zoom, "/" reaches the graph's search ahead of the Specs filter, Enter
// there selects and centres the next match, Esc clears the search and then
// the selection, and a link in a list, the drawer or the chain selects on
// the graph.
func TestPage_TheKeyboardRunsThroughSearchListsAndTheCanvasBox(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="gc-canvas" tabindex="0"`)
	assert.NotContains(t, script, "tpPure.tabStops(", "no canvas node is a tab stop")
	for _, s := range []string{
		`const KEY_PAN = { ArrowLeft: [-PAN_PX, 0], ArrowRight: [PAN_PX, 0], ArrowUp: [0, -PAN_PX], ArrowDown: [0, PAN_PX] };`,
		`ev.target === $("gc-canvas")`, "zoomBy(ZOOM_STEP)", "ev.stopImmediatePropagation();", "}, true);",
		"flyTo(n);", `if (ev.key === "Escape") {`, `ev.key === "Escape" && app.selected() && $("drawer").hidden`,
		"selectEpic(a.dataset.epic, true)", "if (centre && id && P.index[id] !== undefined && G.vs) flyTo(",
	} {
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
const draw = (v) => secs.map((s) => P.waitSection(s, v, 3, T)).map((w) => ({ key: w.key, title: w.title, expanded: w.expanded, shown: w.groups.length, more: w.more }));
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
	for _, s := range []string{"tpPure.waitSection(s, waitView, WAIT_SHOWN, t)", `aria-expanded="' + w.expanded + '"`, `aria-expanded="' + w.more.expanded + '"`, "waitView = tpPure.flipWait(waitView, "} {
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

// TestPage_GraphNodesReadTheirMarkFromTheStatus: each graph node's mark
// comes from tpPure.markState over the export's status, and the legend
// names the three stages after planned.
func TestPage_GraphNodesReadTheirMarkFromTheStatus(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "tpPure.markState(")
	assert.Contains(t, script, "app.markOf(e, ctx.reports[e.milestone])")
	for _, s := range []string{`"mark.in_progress": "in progress"`, `"mark.specced": "specced"`, `"mark.claimed": "claimed"`} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, script, "claimed or under way")
	assert.NotContains(t, script, "function markState(")
}

// TestPage_WaitingEpicsSitBelowTheGraphInGroups: the Waiting epics card
// follows the graph card, draws the groups waitingGroups returns with the
// rest behind "show all", and names the deferrable members its total leaves
// out.
func TestPage_WaitingEpicsSitBelowTheGraphInGroups(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	graphAt := strings.Index(page, `id="graphcard"`)
	waitAt := strings.Index(page, `id="waitcard"`)
	require.GreaterOrEqual(t, waitAt, 0, "the page has a Waiting epics card")
	assert.Greater(t, waitAt, graphAt, "Waiting epics follows the graph card")
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
	assert.Contains(t, script, `"wait.here": { "one": "{n} waits on it here", "other": "{n} wait on it here" }`)
	assert.Contains(t, script, `"wait.hereTitle": "epics of open milestones that wait on it directly"`)
	assert.Contains(t, script, `'<span class="wn num" title="' + esc(t("wait.hereTitle")) + '">' + esc(t("wait.here", { n: gr.epics.length }))`)
}

// TestPage_AMergedBandIsNamedForEveryMilestoneItHolds: a band tp reports
// with others sharing its span, a cycle between milestones, is drawn once
// and labelled with each of its milestones.
func TestPage_AMergedBandIsNamedForEveryMilestoneItHolds(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "const names = [b.milestone].concat(b.with || []);")
	assert.Contains(t, script, `name: names.join(" + ")`)
}

// TestPage_AMilestoneSaysItsInboundCountIsEpicEdges: the Milestones mode
// draws opens-after lines between milestones, while a milestone's inbound
// count counts dependency edges between epics, so the number says "epic
// edges in", never a bare "edges in" that reads as the lines drawn (1.14.1).
func TestPage_AMilestoneSaysItsInboundCountIsEpicEdges(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "in: n.inbound")
	assert.Contains(t, script, `{in} epic edges in"`)
	assert.NotContains(t, script, `{in} edges in"`)
}

// TestPage_TheGraphDrawsOnlyTheDataOfItsKey: the card keeps the key each
// layout was fetched for, clears an error when a new key starts, drops a
// fetch a later one overtook, and draws only the layout fetched for the
// query it shows, so a click or a milestone change never draws, fits or
// centres stale data.
func TestPage_TheGraphDrawsOnlyTheDataOfItsKey(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"G.error = null;", "if (seq !== G.seq) return;", "dataKey: key", "if (!G.data || G.dataKey !== key) return;"} {
		assert.Contains(t, script, s)
	}
}

// TestPage_OneGraphCardDrawsEveryMode is decision 2 of
// spec/backlog/dashboard-graph.md at the page: one card switches between
// Plan, Milestones, Gate closure and Neighbourhood, each fetched from
// /api/layout; it draws the bands, the critical path, any-of edges and the
// neighbourhood's centre, goes full screen, keeps the chain as a
// collapsible text view, and the 1.13.0 Plan map and Dependencies card are
// gone with their drawing code.
func TestPage_OneGraphCardDrawsEveryMode(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{`id="graphcard"`, `data-gmode="plan"`, `data-gmode="milestones"`, `data-gmode="scope"`, `data-gmode="neighbourhood"`, `id="chaintext"`, `data-fs="graphcard"`} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{`id="mapcard"`, `id="chaincard"`, `id="mapsvg"`, `id="gsvg"`, `class="card mapcard retired"`, ".retired"} {
		assert.NotContains(t, page, s)
	}
	for _, s := range []string{"function drawMap(", "function drawGraph(", "function makePort(", "tpPure.placeLayout("} {
		assert.NotContains(t, script, s)
	}
	for _, s := range []string{`"/api/layout?"`, "L.bands", ".critical", ".any_of", ".centre", ".inbound"} {
		assert.Contains(t, script, s)
	}
	chain := strings.Index(page, `id="chaintext"`)
	assert.Greater(t, chain, strings.Index(page, `id="graphcard"`))
	assert.Less(t, chain, strings.Index(page, `id="waitcard"`), "the chain as text sits inside the graph card")
}

// TestPage_TheGraphsHelpIsDefinedOnceAndNamesTheTrade: the card's help is
// one INFO.graph, which names every mode and says the keyboard runs through
// the search, the lists, the drawer and the chain as text.
func TestPage_TheGraphsHelpIsDefinedOnceAndNamesTheTrade(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Equal(t, 1, strings.Count(script, "INFO.graph ="))
	for _, s := range []string{"Plan:", "Milestones:", "Gate closure:", "Neighbourhood:", "the critical path in orange whatever its class", "the search, the lists, the drawer and the chain as text below carry the keyboard"} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheGraphRecordsWhatItDrew is decision 4's draw check at the
// page: after deck.gl renders a frame the card carries the mode drawn and
// the nodes and edges it drew, the nodes the layout's and the edges those
// tpPure.edgeLayers keeps, so a blank canvas reads as zero.
func TestPage_TheGraphRecordsWhatItDrew(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `data-drawn-nodes="0" data-drawn-edges="0"`)
	for _, s := range []string{"onAfterRender: recordDrawn", "card.dataset.drawnNodes = String(d.nodes);", "card.dataset.drawnEdges = String(d.edges);",
		"G.drawing = { mode: P.L.view, nodes: P.nodes.length, edges: sets.drawn };", "pure.edgeLayers(P.links,", "card.dataset.firstDrawMs"} {
		assert.Contains(t, script, s)
	}
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

// TestPage_TheLifecycleRowsSpreadEvenlyToo: the Lifecycle's stage and
// phase cells and its milestone cards take the same balanced, full-width
// rows as the KPI row, fitted when each is drawn and when it changes size
// (1.15.1).
func TestPage_TheLifecycleRowsSpreadEvenlyToo(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{`["#band .pcells", 120]`, `["#msstate .mscells", 170]`, `["kpis", "band", "msstate"].forEach((id) => rowsSeen.observe($(id)));`} {
		assert.Contains(t, script, s)
	}
	assert.Equal(t, 3, strings.Count(script, "fitAllRows();"), "the KPI, band and milestone panels each fit their rows once drawn")
}

// TestPage_FittingRowsWritesOnlyWhatChanged: a fit that finds the same grid
// and spans writes nothing, so the ResizeObserver it feeds settles instead
// of flickering, and the root keeps its scrollbar's gutter, so a scrollbar
// appearing does not narrow the grids and refit them (1.15.1 review).
func TestPage_FittingRowsWritesOnlyWhatChanged(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{"if (box.dataset.cols !== String(grid.columns)) {", "if (c.style.gridColumn !== span) c.style.gridColumn = span;"} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, ":root { scrollbar-gutter: stable; }")
}

// TestPure_TheEpicTotalsNameTheConditions: the Epics tab counts every epic
// while the share and the Lifecycle count work epics, so the totals read as
// one sentence that names the condition epics between them (1.14.1).
func TestPure_TheEpicTotalsNameTheConditions(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.epicTotals([{ id: "a" }, { id: "b" }, { id: "k", condition: true }], T), P.epicTotals([{ id: "a" }], T), P.epicTotals([{ id: "k", condition: true }], T)]);`)
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
	assert.Contains(t, script, `t("kpi.share"), "", totals`)
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
	for _, s := range []string{"tpPure.milestoneHTML(msd, gateOf(g, msd.milestone), dc, words(THRESHOLD_LABEL), t)", "tpPure.thresholdLine(m, gateOf(g, m.milestone), words(THRESHOLD_LABEL), t)", `$("dkind").textContent = msd ? t("drawer.milestone") : t("drawer.epic");`} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, script, "function milestoneHTML(", "the detail is built in pure.js, where its escaping is tested")
}

// TestPage_TheDrawersLabelsNameWhatItShows: in milestone mode every label
// the drawer's header carries names a milestone, the region, the close
// button and the info button alike (1.15.1 review).
func TestPage_TheDrawersLabelsNameWhatItShows(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="dinfo"`)
	for _, s := range []string{`$("drawer").setAttribute("aria-label", msd ? t("drawer.msLabel") : t("drawer.epicLabel"));`, `$("dclose").setAttribute("aria-label", msd ? t("drawer.msClose") : t("drawer.epicClose"));`,
		`$("dinfo").setAttribute("aria-label", msd ? t("drawer.msAbout") : t("drawer.epicAbout"));`, `"drawer.msLabel": "Milestone detail"`, `"drawer.msClose": "Close the milestone detail"`, `"drawer.msAbout": "About the milestone detail"`} {
		assert.Contains(t, script, s)
	}
}

// TestPure_AMilestonesDetailEscapesAndReadsItsGateInFull: the drawer's
// milestone detail is built in pure.js: the gate's state, the threshold
// and the purpose in full, and the described count, every text escaped,
// so markup in a threshold or a purpose reads as text (1.15.1 review).
func TestPure_AMilestonesDetailEscapesAndReadsItsGateInFull(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const L = { written: "threshold written", absent: "no threshold", placeholder: "placeholder threshold", met: "threshold met" };
out([
  P.milestoneHTML({ milestone: "A<2>", open: false, waiting_on: ["A1"], state: { threshold: "written", members: 6, members_done: 1 } }, { threshold: " p95 <b>fast</b> ", purpose: " proves <b>it</b> " }, { described: 1, all: 1 }, L, T),
  P.milestoneHTML({ milestone: "B", open: true, state: { threshold: "written" }, gate: { threshold: "from export" } }, { threshold: "from graph" }, { described: 0, all: 2 }, L, T),
]);`)
	assert.JSONEq(t, `[
		"<h3>A&lt;2&gt;</h3><dl><dt>Gate</dt><dd>closed, waits on A1; 1 of 6 members done</dd><dt>Threshold</dt><dd>threshold written: p95 &lt;b&gt;fast&lt;/b&gt;</dd><dt>Purpose</dt><dd>proves &lt;b&gt;it&lt;/b&gt;</dd><dt>Described</dt><dd>1 of 1 epic described</dd></dl>",
		"<h3>B</h3><dl><dt>Gate</dt><dd>open; 0 of 0 members done</dd><dt>Threshold</dt><dd>threshold written: from export</dd><dt>Purpose</dt><dd>none written</dd><dt>Described</dt><dd>0 of 2 epics described</dd></dl>"]`, got)
}

// TestPure_TheCardAndTheDrawerReadOneThreshold: a card's threshold line
// and the drawer read the threshold text from one place, the export's
// m.gate, else the graph's gate, so a written threshold the export omits
// still shows its words on the card (1.15.1 review).
func TestPure_TheCardAndTheDrawerReadOneThreshold(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const L = { written: "threshold written", absent: "no threshold", placeholder: "placeholder threshold", met: "threshold met" };
out([
  P.thresholdLine({ state: { threshold: "written" } }, { threshold: " graph text " }, L, T),
  P.thresholdLine({ state: { threshold: "written" }, gate: { threshold: "export" } }, { threshold: "graph" }, L, T),
  P.thresholdLine({ state: { threshold: "met" } }, { threshold: "x" }, L, T),
  P.thresholdLine({ state: {} }, {}, L, T),
  P.thresholdOf({ gate: { threshold: "export" } }, { threshold: "graph" }),
  P.thresholdOf({}, undefined),
]);`)
	assert.JSONEq(t, `["threshold: graph text", "threshold: export", "threshold met", "no threshold", "export", ""]`, got)
}

// TestPure_TheDrawerMovesBetweenEpicAndMilestone: the drawer's state, the
// milestone shown and the epic selected, moves by drawerNext: a card opens
// its milestone, the same card again closes it, another card switches, an
// epic takes the drawer over and a card takes it back, a milestone a push
// no longer carries is dropped and stays dropped when it returns, and an
// epic toggles or closes as before; a card reads expanded only while its
// milestone is the one shown (1.15.1 review).
func TestPure_TheDrawerMovesBetweenEpicAndMilestone(t *testing.T) {
	t.Parallel()
	got := runPure(t, `let s = { detail: null, selected: null };
const steps = [];
const go = (a) => { s = P.drawerNext(s, a); steps.push([s.detail, s.selected, P.cardExpanded(s, "A2")]); };
go({ type: "milestone", id: "A2" });
go({ type: "milestone", id: "B2" });
go({ type: "milestone", id: "B2" });
go({ type: "milestone", id: "A2" });
go({ type: "present", milestones: ["A2", "B2"] });
go({ type: "epic", id: "e1" });
go({ type: "milestone", id: "A2" });
go({ type: "present", milestones: ["B2"] });
go({ type: "present", milestones: ["A2", "B2"] });
go({ type: "epic", id: "e1" });
go({ type: "epic", id: "e1" });
go({ type: "epic", id: "e2" });
go({ type: "epic", id: null });
out(steps);`)
	assert.JSONEq(t, `[["A2", null, true], ["B2", null, false], [null, null, false], ["A2", null, true], ["A2", null, true],
		[null, "e1", false], ["A2", null, true], [null, null, false], [null, null, false],
		[null, "e1", false], [null, null, false], [null, "e2", false], [null, null, false]]`, got)
}

// TestPage_TheDrawerFollowsItsReducer: the page moves the drawer only by
// tpPure.drawerNext, drops a milestone the export no longer carries before
// drawing, so a later push cannot reopen it and take focus, and marks the
// cards expanded by tpPure.cardExpanded (1.15.1 review).
func TestPage_TheDrawerFollowsItsReducer(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.drawerNext(", `moveDrawer({ type: "present", milestones: `, `moveDrawer({ type: "epic", id });`, `moveDrawer({ type: "milestone", id });`, "tpPure.cardExpanded(drawerState(), b.dataset.ms)"} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, script, "b.dataset.ms === msd.milestone")
}

// TestPage_TheMilestoneShownLivesInTheAddress: the milestone the drawer
// shows is the address's ms, a place key, written when a card opens it and
// read back with the selected epic, so a reload keeps it and back and
// forward close and reopen it instead of leaving it stale (1.15.1 review).
func TestPage_TheMilestoneShownLivesInTheAddress(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, re := range []string{`const URL_KEYS = \[[^\]]*"ms"[^\]]*\];`, `const PLACE_KEYS = \[[^\]]*"ms"[^\]]*\];`} {
		assert.Regexp(t, re, script)
	}
	for _, s := range []string{`put("ms", msView.detail);`, `msView.detail = p.get("ms") || null;`, "moveDrawer({ type: \"milestone\", id });\n    drawSelected();\n    syncURL();"} {
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
	assert.Contains(t, script, `const FOCUSABLE = "[data-epic], [data-ms], [data-wsec], [data-wall]";`)
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
	got := runPure(t, `out([P.closureDescribed([{ id: "a" }, { id: "b" }, { id: "c" }], new Set(["a", "c", "z"]), T), P.closureDescribed([], new Set(["a"]), T)]);`)
	assert.JSONEq(t, `["2 of 3 in the closure described", "0 of 0 in the closure described"]`, got)
}

// TestPage_TheClosureHeaderSaysHowManyAreDescribed: the graph card's
// count line in Gate closure carries closureDescribed's sentence.
func TestPage_TheClosureHeaderSaysHowManyAreDescribed(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "pure.closureDescribed(L.nodes, described, app.tr())")
}

// TestPage_MarksDescribedEpicsAndCountsThemPerMilestone: a described epic
// carries a mark after its id on the graph, and each milestone's entry says
// how many of its epics are described.
func TestPage_MarksDescribedEpicsAndCountsThemPerMilestone(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"tpPure.describedCounts(", `ctx.described.has(n.id) ? " ¶" : ""`, " described"} {
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
	assert.Contains(t, script, `t("slice.set", { name: k.name })`)
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
	assert.Contains(t, script, `<td data-l="' + esc(col.state) + '">`)
}

// TestPure_TheUpdateTimeReadsRelative: the header's update time reads as
// how long ago tp last pushed, in the largest whole unit up to hours, days
// past two of them, as the locale writes it (1.17.0).
func TestPure_TheUpdateTimeReadsRelative(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const ago = [0, 9999, 10000, 59000, 60000, 3599000, 3600000, 7300000, 172800000, -5];
const tr = P.translator({ "time.justNow": "az önce" }, "tr");
out({ en: ago.map((ms) => P.relTime(ms, T)), tr: [0, 10000, 3600000, 172800000].map((ms) => P.relTime(ms, tr)) });`)
	assert.JSONEq(t, `{"en": ["just now", "just now", "10 sec. ago", "59 sec. ago", "1 min. ago", "59 min. ago", "1 hr. ago", "2 hr. ago", "2 days ago", "just now"],
		"tr": ["az önce", "10 sn. önce", "1 sa. önce", "2 gün önce"]}`, got)
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

// TestPure_TheAddressStaysOnThePagesOriginUnderADoubledSlash: at
// http://127.0.0.1:7377// the address syncURL writes resolves on the page's
// own origin and keeps its path and query, where "//#…" names another
// origin and history.replaceState throws a SecurityError.
func TestPure_TheAddressStaysOnThePagesOriginUnderADoubledSlash(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(["http://127.0.0.1:7377//?x=1", "http://127.0.0.1:7377/"].map((base) => {
  try {
    const u = new URL(P.addressURL("tab=epics"), base);
    return [u.origin, u.pathname + u.search + u.hash];
  } catch (err) {
    return [String(err.message)];
  }
}));`)
	assert.JSONEq(t, `[["http://127.0.0.1:7377", "//?x=1#tab=epics"], ["http://127.0.0.1:7377", "/#tab=epics"]]`, got)
}

// TestPage_AHiddenTabClosesItsStreamAndReopensItVisible: the page closes its
// EventSource when the tab is hidden and opens a fresh one when it is
// visible again, so hidden tabs hold none of the browser's six connections
// per host and a returning tab gets the current state the server sends a
// new stream first.
func TestPage_AHiddenTabClosesItsStreamAndReopensItVisible(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `document.addEventListener("visibilitychange"`)
	assert.Contains(t, script, "tpPure.streamStep(document.hidden, source !== null)")
	assert.Contains(t, script, "source.close();")
	assert.Contains(t, script, "source = new EventSource(STREAM);")
	assert.Contains(t, script, "} else if (step === \"open\") {\n      openStream();\n    }", "the open step opens a fresh stream")
	assert.Contains(t, script, "if (!document.hidden) openStream();", "a tab loaded visible opens its stream")
}

// TestPure_TheStreamClosesHiddenAndOpensVisible: a hidden tab with an open
// stream closes it, a visible one without a stream opens one, and the other
// two states leave it as it is.
func TestPure_TheStreamClosesHiddenAndOpensVisible(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([[true, true], [true, false], [false, false], [false, true]].map(([hidden, open]) => P.streamStep(hidden, open)));`)
	assert.JSONEq(t, `["close", null, "open", null]`, got)
}

// TestPage_SyncURLWritesTheRelativeAddress: syncURL builds its address with
// tpPure.addressURL from the place alone, so the page cannot go back to
// prefixing location.pathname.
func TestPage_SyncURLWritesTheRelativeAddress(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "const url = tpPure.addressURL(next.toString());")
	assert.NotContains(t, script, "location.pathname + location.search")
}

// TestPage_AReopenedStreamSaysConnectingAndPausedHasItsOwnMark: opening a
// stream sets the indicator to Connecting, so a tab shown again does not
// keep reading Paused until the stream opens, and Paused has a mark of its
// own.
func TestPage_AReopenedStreamSaysConnectingAndPausedHasItsOwnMark(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, "function openStream() {\n    connState(\"connecting\");\n")
	assert.Contains(t, script, `paused: tk("conn.paused")`)
	assert.Contains(t, page, `.conn[data-state="paused"]::before {`)
}

package dashboard_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ownersJS is a declared owners set for the owner tests: a named owner, an
// unnamed one, and one whose handle is the word "unowned", which must stay
// an owner like any other (spec/backlog/epic-owner.md).
const ownersJS = `const owners = [{ handle: "ana", name: "Ana A." }, { handle: "ben" }, { handle: "unowned", name: "U" }];`

// TestPure_AnOwnerIsNamedSlottedAndSelectedByItsPlaceInTheSet: a label is
// the display name or the handle; a slot is the position in the declared
// set; a selection is a declared handle or null for the unowned, so a handle
// spelled "unowned" is selected as itself and never as the unowned.
func TestPure_AnOwnerIsNamedSlottedAndSelectedByItsPlaceInTheSet(t *testing.T) {
	t.Parallel()
	got := runPure(t, ownersJS+`
out([
  P.ownerLabel(owners, "ana"), P.ownerLabel(owners, "ben"), P.ownerLabel(owners, "zed"),
  P.ownerSlot(owners, "ana"), P.ownerSlot(owners, "unowned"), P.ownerSlot(owners, null), P.ownerSlot(owners, "zed"),
  P.ownerSelected(["unowned"], owners, "unowned"), P.ownerSelected(["unowned"], owners, null), P.ownerSelected([], owners, null),
  P.ownerSelected(["ben", "ana"], owners, "ana"), P.ownerSelected(["ben"], owners, "ana"), P.ownerSelected(["ben"], owners, undefined),
]);`)
	assert.JSONEq(t, `["Ana A.", "ben", "zed", 0, 2, null, null, true, false, true, true, false, true]`, got)
}

// TestPure_TheOwnerLegendCountsByMembershipNeverByPaint: an epic with two
// owners counts under each though the map paints it in the first one's
// colour; owners past the eighth share one counted "other" row, counting
// each epic naming any of them once; the unowned are counted on their own.
func TestPure_TheOwnerLegendCountsByMembershipNeverByPaint(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const owners = Array.from({ length: 10 }, (_, i) => ({ handle: "h" + i }));
owners[0].name = "Zero";
const epics = [{ owner: ["h0"] }, { owner: ["h1", "h0"] }, { owner: ["h8"] }, { owner: ["h9", "h8"] }, {}, { owner: [] }];
out(P.ownerLegend(owners, epics));`)
	assert.JSONEq(t, `{"owners": [{"slot": 0, "label": "Zero", "n": 2}, {"slot": 1, "label": "h1", "n": 1}],
		"other": {"owners": 2, "n": 2}, "unowned": 2}`, got)
}

// TestPure_ConditionsGroupUnderEachOwnerTheUnownedLast: a condition with two
// owners is listed under each, so the groups can sum past the conditions and
// total counts them once; the unowned group comes last, keyed by null, and
// holds a condition naming only handles the set does not declare, so none
// is hidden; a handle spelled "unowned" is an owner's group.
func TestPure_ConditionsGroupUnderEachOwnerTheUnownedLast(t *testing.T) {
	t.Parallel()
	got := runPure(t, ownersJS+`
const own = { c1: ["ben", "ana"], c2: ["ana"], c3: [], c4: ["zed"], c5: ["unowned"] };
out(P.ownerGroups(owners, ["c1", "c2", "c3", "c4", "c5"], (id) => own[id]));`)
	assert.JSONEq(t, `{"total": 5, "groups": [
		{"owner": 0, "label": "Ana A.", "ids": ["c1", "c2"]},
		{"owner": 1, "label": "ben", "ids": ["c1"]},
		{"owner": 2, "label": "U", "ids": ["c5"]},
		{"owner": null, "label": null, "ids": ["c3", "c4"]}]}`, got)
}

// TestPure_UnownedMeansNoDeclaredOwnerEverywhere: an epic naming only a
// handle the set does not declare (a lint error the page still draws) is
// unowned to the legend and to the map's selection, as it is to the
// operator groups, so it is counted and lit once and never hidden.
func TestPure_UnownedMeansNoDeclaredOwnerEverywhere(t *testing.T) {
	t.Parallel()
	got := runPure(t, ownersJS+`
out([P.ownerLegend(owners, [{ owner: ["zed"] }, { owner: [] }]).unowned,
  P.ownerSelected(["zed"], owners, null), P.ownerSelected(["zed", "ben"], owners, null)]);`)
	assert.JSONEq(t, `[2, true, false]`, got)
}

// TestPure_OwnerMarkupIsEscapedWhereItIsWritten: a handle and a display
// name are author content, so the drawer's owner list and the map's owner
// selector write markup in them as text; the selector's values are places
// in the set and "u" for the unowned, never a handle.
func TestPure_OwnerMarkupIsEscapedWhereItIsWritten(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const owners = [{ handle: "<b>h</b>" }, { handle: "ana", name: "<img src=x onerror=1>" }];
out([P.ownerNamesHTML(owners, ["ana", "<b>h</b>"]), P.ownerOptionsHTML(owners, "<b>h</b>", T),
  [P.ownerSel("", owners) === undefined, P.ownerSel("u", owners) === null, P.ownerSel("1", owners) === "ana"]]);`)
	assert.JSONEq(t, `[
		"<span class=\"downer\">&lt;img src=x onerror=1&gt;</span>, <span class=\"downer\">&lt;b&gt;h&lt;/b&gt;</span>",
		"<option value=\"\">All owners</option><option value=\"0\" selected>&lt;b&gt;h&lt;/b&gt;</option><option value=\"1\">&lt;img src=x onerror=1&gt;</option><option value=\"u\">Unowned</option>",
		[true, true, true]
	]`, got)
}

// TestPage_TheDrawerListsTheEpicsOwnersOnlyWhenOwnersAreDeclared: the
// epic drawer's fields carry an owner row, its owners in order through the
// escaping helper, and only when the graph declares owners and the epic
// names one, so a graph without owners draws the drawer it drew before.
func TestPage_TheDrawerListsTheEpicsOwnersOnlyWhenOwnersAreDeclared(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `owners.length && (ge.owner || []).length ? [t("drawer.f.owner"), tpPure.ownerNamesHTML(owners, ge.owner), true] : null,`)
	assert.Contains(t, script, `const owners = (g.graph && g.graph.owners) || [];`)
}

// TestPage_TheOperatorsConditionsGroupByWhoseDecisionItIs: with owners
// declared, the conditions waiting on a decision are grouped under "waiting
// on <owner>" by tpPure.ownerGroups, the unowned last, each heading counted
// and escaped, after a line stating the distinct conditions; without owners
// they stay one plain list.
func TestPage_TheOperatorsConditionsGroupByWhoseDecisionItIs(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		`if (!owners.length || !ids.length) return rows(ids);`,
		`const by = tpPure.ownerGroups(owners, ids, (id) => (graphEpic(v.g, id) || {}).owner);`,
		`esc(t("op.toDecide", { n: by.total }))`,
		`esc(gr.owner === null ? t("op.unowned") : t("op.waitingOn", { owner: gr.label }))`,
		`const conds = conditionRows(v, done);`,
	} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, ".list .ogroup {")
}

// TestPage_EveryOwnerColourReadsAndStaysApart: the owner palette, eight
// slots, "other" and the unowned, is a token set on the light theme and
// both dark ones; drawn through tpPure.graphPalette, the ink reads on every
// owner fill at 4.5:1, every owner hue reads on the canvas at 3:1, and no
// two of the eight slots come closer than a colour difference of 12.
func TestPage_EveryOwnerColourReadsAndStaysApart(t *testing.T) {
	t.Parallel()
	css := string(served(t, "/style.css"))
	for _, over := range []string{"", darkByControl, darkBySystem} {
		tok, err := json.Marshal(themeTokens(t, css, over))
		require.NoError(t, err)
		got := runPure(t, `const tok = `+string(tok)+`;
const pal = P.graphPalette(tok);
const r = (a, b) => Math.round(P.contrast(a, b) * 100) / 100;
let near = 99;
for (let a = 0; a < P.OWNER_SLOTS; a++) for (let b = a + 1; b < P.OWNER_SLOTS; b++) near = Math.min(near, P.deltaE(pal.owner.line[a], pal.owner.line[b]));
out({ n: pal.owner.fill.length, text: Math.min(...pal.owner.fill.map((f) => r(pal.ink, f))), line: Math.min(...pal.owner.line.map((l) => r(l, pal.page))), near: Math.round(near * 10) / 10,
  tokens: P.OWNER_TOKENS.every((name) => /^#[0-9a-f]{6}$/i.test(tok[name] || "")) });`)
		var res struct {
			N      int     `json:"n"`
			Text   float64 `json:"text"`
			Line   float64 `json:"line"`
			Near   float64 `json:"near"`
			Tokens bool    `json:"tokens"`
		}
		require.NoError(t, json.Unmarshal([]byte(got), &res), got)
		assert.True(t, res.Tokens, "theme %q: every owner token is a colour", over)
		assert.Equal(t, 10, res.N, "theme %q: eight slots, other and the unowned", over)
		assert.GreaterOrEqual(t, res.Text, 4.5, "theme %q: ink on an owner fill", over)
		assert.GreaterOrEqual(t, res.Line, 3.0, "theme %q: an owner hue on the canvas", over)
		assert.GreaterOrEqual(t, res.Near, 12.0, "theme %q: the closest two owner slots", over)
	}
}

// TestPage_TheMapSelectsAndColoursByOwner: with owners declared and epics
// drawn, the toolbar shows an owner selector and a colour-by-owner switch;
// the selection dims the other epics through nodeAlpha as search and the
// stage do; the switch fills a node in its first owner's colour while its
// border keeps the state mark, and the legend then shows the owners, counted
// by membership, and the state marks under separate headings; an epic with
// more than one owner carries "+N"; without owners none of it shows.
func TestPage_TheMapSelectsAndColoursByOwner(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		`<label class="gpick" id="gc-owner-pick" hidden>`, `<select id="gc-owner"></select>`,
		`id="gc-ocolour" aria-pressed="false" hidden`,
	} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{
		`const owned = ownersOf(g).length > 0 && G.mode !== "milestones";`,
		`$("gc-owner").innerHTML = pure.ownerOptionsHTML(ownersOf(g), G.ownerSel, app.tr());`,
		`if (P.owners.length && !pure.ownerSelected(n.owner, P.owners, G.ownerSel)) return 50;`,
		`if (ownerPainted(P)) return [...c.owner.fill[ownerSlotOf(P, n)], nodeAlpha(P, hits, n)];`,
		`pure.ownerLegend(P.owners, P.nodes)`,
		`(owner.length > 1 ? " +" + (owner.length - 1) : "")`,
		`G.ownerSel = pure.ownerSel(ev.target.value, ownersOf(app.data("epic-export")));`,
		`G.ownerSel = pure.ownerResolve(ownersOf(g), G.ownerSel);`,
		`G.ownerColour = !G.ownerColour;`,
		`pure.ownerSelKey(G.ownerSel), G.ownerColour`,
		`escape(t("graph.legend.owner"))`, `escape(t("graph.legend.state"))`,
	} {
		assert.Contains(t, script, s)
	}
}

// TestPure_AnOwnerSelectionFollowsItsHandleAcrossPushes: the map keeps the
// selected owner by handle, the unowned as null and every epic as
// undefined, so a push that reorders the set keeps the same owner and one
// that drops it resets to every epic; a handle spelled "undefined" or
// "unowned" stays an owner, apart from those two in the redraw key too.
func TestPure_AnOwnerSelectionFollowsItsHandleAcrossPushes(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const a = [{ handle: "ana" }, { handle: "ben" }, { handle: "cem" }];
const dropped = [{ handle: "ana" }, { handle: "ben" }];
const moved = [{ handle: "cem" }, { handle: "ana" }, { handle: "ben" }];
const odd = [{ handle: "undefined" }, { handle: "unowned" }];
const sel = P.ownerSel("2", a);
out([sel, P.ownerResolve(dropped, sel) === undefined, P.ownerResolve(moved, sel),
  P.ownerSelected(["cem"], moved, P.ownerResolve(moved, sel)), P.ownerOptionsHTML(moved, "cem", T).includes('value="0" selected'),
  P.ownerSel("u", a) === null, P.ownerResolve(a, null) === null, P.ownerSel("", a) === undefined,
  P.ownerSel("1", odd), P.ownerSelected(["unowned"], odd, "unowned"), P.ownerSelected([], odd, "unowned"),
  P.ownerSelKey(undefined) !== P.ownerSelKey("undefined"), P.ownerSelKey(null) !== P.ownerSelKey("unowned")]);`)
	assert.JSONEq(t, `["cem", true, "cem", true, true, true, true, true, "unowned", true, false, true, true]`, got)
}

// TestPage_OwnerColourKeepsTheStateBorderFarOut: with the fill in the
// owner's colour the border is the only state mark, so the far view, which
// otherwise draws no border, keeps one while owners paint the nodes.
func TestPage_OwnerColourKeepsTheStateBorderFarOut(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `return G.detail === "far" && !ownerPainted(P) ? 0 : 1;`)
}

// TestPage_TheSelectionWinsOverTheOwnerDim: a selected epic and its chain
// are drawn as the selection draws them even when the owner selection would
// dim them, so clicking another owner's epic never leaves it faint.
func TestPage_TheSelectionWinsOverTheOwnerDim(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `    if (P.sel >= 0) return inChain(P, n.i) ? pure.chainAlpha(stepsOf(P, n.i), G.whole) : 40;
    if (P.owners.length && !pure.ownerSelected(n.owner, P.owners, G.ownerSel)) return 50;`)
}

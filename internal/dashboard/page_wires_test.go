package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stepsLayout is a Steps panel measured the way the page measures it: three
// columns of cards in step rows, each step under its header, the shape of the
// field report where edges from step 1 to step 3 crossed the cards of step 2.
const stepsLayout = `
const col = (c) => 24 + c * 292;
const card = (c, y) => ({ x: col(c), y, w: 280, h: 112 });
const C = {
  s1a: card(0, 44), s1b: card(1, 44), s1c: card(2, 44),
  s2a: card(0, 208), s2b: card(1, 208), s2c: card(2, 208),
  s2d: card(0, 332), s2e: card(1, 332),
  s3a: card(0, 496),
};
const heads = [{ x: 24, y: 16, w: 246, h: 20 }, { x: 24, y: 180, w: 330, h: 20 }, { x: 24, y: 468, w: 246, h: 20 }];
const edges = [["s1a", "s3a"], ["s1b", "s3a"], ["s1a", "s2c"], ["s1a", "s2a"], ["s2e", "s3a"], ["s1c", "s2d"], ["s2b", "s3a"]];
const route = (opts) => edges.map(([f, to]) => {
  const obstacles = Object.keys(C).filter((k) => k !== f && k !== to).map((k) => C[k]).concat(heads);
  return { f, to, pts: P.routeWire(C[f], C[to], obstacles, opts), obstacles };
});
const inside = (s, r) => {
  const x0 = Math.min(s[0].x, s[1].x), x1 = Math.max(s[0].x, s[1].x);
  const y0 = Math.min(s[0].y, s[1].y), y1 = Math.max(s[0].y, s[1].y);
  return x0 < r.x + r.w && x1 > r.x && y0 < r.y + r.h && y1 > r.y;
};
const faults = (routes) => {
  const bad = [];
  routes.forEach((r) => {
    const p = r.pts;
    const a = C[r.f], b = C[r.to];
    if (p[0].x !== a.x + a.w / 2 || p[0].y !== a.y + a.h) bad.push(r.f + "->" + r.to + " starts off its source's bottom");
    const z = p[p.length - 1];
    if (z.x !== b.x + b.w / 2 || z.y !== b.y) bad.push(r.f + "->" + r.to + " ends off its target's top");
    for (let i = 1; i < p.length; i++) {
      const s = [p[i - 1], p[i]];
      if (s[0].x !== s[1].x && s[0].y !== s[1].y) bad.push(r.f + "->" + r.to + " has a slanted leg");
      r.obstacles.forEach((o) => { if (inside(s, o)) bad.push(r.f + "->" + r.to + " crosses " + JSON.stringify(o)); });
    }
  });
  return bad;
};
`

// A routed wire leaves its source's bottom, enters its target's top, runs in
// straight horizontal and vertical legs, and none of its legs touches a card
// or a step header other than its own two ends.
func TestPure_AStepsWireNeverCrossesAnotherCardOrHeader(t *testing.T) {
	t.Parallel()
	got := runPure(t, stepsLayout+`out(faults(route()));`)
	assert.Equal(t, "[]", got)
}

// The offsets that spread wires sharing a gap never push one into a card:
// however large the asked shift, it stays inside the gap it was given.
func TestPure_AWireOffsetStaysInsideItsGap(t *testing.T) {
	t.Parallel()
	got := runPure(t, stepsLayout+`out(faults(route({ dx: 40, dy: 40 })).concat(faults(route({ dx: -40, dy: -40 }))));`)
	assert.Equal(t, "[]", got)
}

// A wire to the card right under its source stays between the columns: a
// step header beside the drop moves it to the first free line past the
// header's words, never out to the panel's margin.
func TestPure_AWireToTheNextRowStaysBetweenTheColumns(t *testing.T) {
	t.Parallel()
	got := runPure(t, stepsLayout+`out(route().find((r) => r.f === "s1a" && r.to === "s2a").pts.every((pt) => pt.x >= 24));`)
	assert.Equal(t, "true", got)
}

// wirePath turns a route's corners into short curves, so a turn reads as a
// bend and never as two lines meeting, and keeps every point of the route.
func TestPure_AWirePathRoundsEveryCorner(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(P.wirePath([{ x: 0, y: 0 }, { x: 0, y: 20 }, { x: 40, y: 20 }, { x: 40, y: 60 }], 6));`)
	d := strings.Trim(got, `"`)
	assert.True(t, strings.HasPrefix(d, "M0 0"), d)
	assert.Equal(t, 2, strings.Count(d, "Q"), "one curve per corner: %s", d)
	assert.True(t, strings.HasSuffix(d, "L40 60"), d)
}

// The wires are drawn under the steps, so a card's surface covers any wire
// that reaches it, and a done card's tinted surface is opaque, not a
// see-through tint over the wires.
func TestPage_StepsWiresLieUnderTheCards(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, ".wires { position: absolute; left: 0; top: 0; z-index: 0;")
	assert.Contains(t, page, ".step { position: relative; z-index: 1;")
	assert.Contains(t, page, `.tcard[data-s="done"] { --c: var(--icon-success); background: linear-gradient(var(--fill-weak), var(--fill-weak)) var(--surface);`)
	assert.Contains(t, script, `box.querySelectorAll(".step-h > *")`, "only a header's words are an obstacle")
}

// A done card keeps its tint under hover and press: the hover layer is
// drawn over the done tint, not in place of it.
func TestPage_ADoneCardKeepsItsTintOnHover(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, `.tcard[data-s="done"]:hover { background-image: linear-gradient(var(--fill-hover), var(--fill-hover)), linear-gradient(var(--fill-weak), var(--fill-weak)); }`)
	assert.Contains(t, page, `.tcard[data-s="done"]:active { background-image: linear-gradient(var(--fill-press), var(--fill-press)), linear-gradient(var(--fill-weak), var(--fill-weak)); }`)
}

// wirePoints is a route as the points deck.gl draws a path through: the
// route's ends kept, each corner a few points on a short curve, and no point
// further than the radius from the straight route.
func TestPure_WirePointsRoundACornerAndKeepTheEnds(t *testing.T) {
	t.Parallel()
	got := runPure(t, `
const pts = P.wirePoints([{ x: 0, y: 0 }, { x: 0, y: 20 }, { x: 40, y: 20 }], 6);
const off = pts.filter(([x, y]) => Math.min(Math.abs(x), Math.abs(y - 20)) > 6 + 1e-9).length;
out({ first: pts[0], last: pts[pts.length - 1], many: pts.length > 4, off });`)
	assert.JSONEq(t, `{"first":[0,0],"last":[40,20],"many":true,"off":0}`, got)
}

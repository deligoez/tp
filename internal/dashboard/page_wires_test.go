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
const routes = edges.map(([f, to]) => {
  const obstacles = Object.keys(C).filter((k) => k !== f && k !== to).map((k) => C[k]).concat(heads);
  return { f, to, pts: P.routeWire(C[f], C[to], obstacles) , obstacles };
});
`

// A routed wire leaves its source's bottom, enters its target's top, runs in
// straight horizontal and vertical legs, and none of its legs touches a card
// or a step header other than its own two ends.
func TestPure_AStepsWireNeverCrossesAnotherCardOrHeader(t *testing.T) {
	t.Parallel()
	got := runPure(t, stepsLayout+`
const inside = (s, r) => {
  const x0 = Math.min(s[0].x, s[1].x), x1 = Math.max(s[0].x, s[1].x);
  const y0 = Math.min(s[0].y, s[1].y), y1 = Math.max(s[0].y, s[1].y);
  return x0 < r.x + r.w && x1 > r.x && y0 < r.y + r.h && y1 > r.y;
};
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
out(bad);
`)
	assert.Equal(t, "[]", got)
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

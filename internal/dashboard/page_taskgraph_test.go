package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// taskFixture is a spec's tasks the shape of the Steps panel: a chain that
// skips a step (a -> d), a fan-in and a task with no dependency at all.
const taskFixture = `
const TS = [
  { id: "a" }, { id: "b" }, { id: "c", depends_on: ["a"] },
  { id: "d", depends_on: ["a", "c"] }, { id: "e", depends_on: ["b"] },
  { id: "f" }, { id: "g", depends_on: ["d", "e"] },
];
`

// A task's chain is everything it waits on, however far up, and everything
// that waits on it, however far down; the task itself is in neither.
func TestPure_TaskChainIsTransitiveBothWays(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskFixture+`out([P.taskChain(TS, "d"), P.taskChain(TS, "b"), P.taskChain(TS, "zz")]);`)
	assert.JSONEq(t, `[{"up":["a","c"],"down":["g"]},{"up":[],"down":["e","g"]},{"up":[],"down":[]}]`, got)
}

// One camera zoom scales a card and its text alike. A zoom step starts from
// whatever deck.gl's controller handed back, zoomX and zoomY or an array zoom
// included, and leaves one scalar zoom, so a card's width and its label's size
// on screen keep the ratio they have in the world at every level: a pinch
// never grows the words alone.
func TestPure_TaskGraphZoomScalesCardsAndTextAlike(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const card = 240, text = 14;
let vs = { target: [100, 50, 0], zoom: [0.7, 0.7], zoomX: 0.2, zoomY: 0.2 };
const seen = [];
[0.5, -1.25, 2, -3].forEach((dz) => {
  vs = P.zoomCamera(vs, dz, 30, -20);
  seen.push({ scalar: typeof vs.zoom === "number" && !("zoomX" in vs) && !("zoomY" in vs),
    ratio: Math.round(P.onScreen(vs, card) / P.onScreen(vs, text) * 1000) / 1000 });
});
out(seen);`)
	assert.JSONEq(t, `[{"scalar":true,"ratio":17.143},{"scalar":true,"ratio":17.143},{"scalar":true,"ratio":17.143},{"scalar":true,"ratio":17.143}]`, got)
}

// A zoom step keeps the world point under the pointer where it was on
// screen.
func TestPure_TaskGraphZoomKeepsThePointUnderThePointer(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const vs = { target: [100, 50, 0], zoom: 0 };
const next = P.zoomCamera(vs, 1, 40, -10);
const s0 = Math.pow(2, vs.zoom), s1 = Math.pow(2, next.zoom);
out([vs.target[0] + 40 / s0, vs.target[1] - 10 / s0, next.target[0] + 40 / s1, next.target[1] - 10 / s1, next.zoom]);`)
	assert.JSONEq(t, `[140,40,140,40,1]`, got)
}

// An edge leaves its source's bottom and enters its target's top straight
// down: the curve starts and ends on the two points with vertical tangents.
func TestPure_TaskEdgeCurveHasVerticalTangents(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const c = P.vcurve(10, 0, 200, 100, 8);
const steep = (p, q) => Math.abs(q[0] - p[0]) < Math.abs(q[1] - p[1]);
out({ n: c.length, first: c[0], last: c[c.length - 1],   up: steep(c[0], c[1]), down: steep(c[c.length - 2], c[c.length - 1]) });`)
	assert.JSONEq(t, `{"n":9,"first":[10,0],"last":[200,100],"up":true,"down":true}`, got)
}

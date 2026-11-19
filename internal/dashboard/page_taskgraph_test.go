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
const GEO = { w: 100, h: 40, gx: 10, gy: 10, head: 30, sgap: 20 };
`

// The graph's steps are the Steps panel's: each task one step below the last
// task it waits on, in the export's order within a step.
func TestPure_TaskGraphStepsMatchTheStepsPanel(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskFixture+`out(P.taskSteps(TS));`)
	assert.JSONEq(t, `[["a","b","f"],["c","e"],["d"],["g"]]`, got)
}

// A step wraps after cols tasks, its rows stacked under its header, and no
// two nodes overlap.
func TestPure_TaskGraphLayoutWrapsAStepAndNeverOverlaps(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskFixture+`
const L = P.taskGraphLayout(TS, 2, GEO);
const at = {}; L.nodes.forEach((n) => { at[n.id] = n; });
const overlap = L.nodes.some((p, i) => L.nodes.some((q, j) => j > i && p.x < q.x + q.w && q.x < p.x + p.w && p.y < q.y + q.h && q.y < p.y + p.h));
out({ a: [at.a.x, at.a.y], b: [at.b.x, at.b.y], f: [at.f.x, at.f.y], c: [at.c.x, at.c.y], steps: L.steps.map((s) => s.y), overlap,
  bounds: L.bounds });`)
	assert.JSONEq(t, `{"a":[0,30],"b":[110,30],"f":[0,80],"c":[0,170],
		"steps":[0,140,230,320],"overlap":false,"bounds":{"x0":0,"y0":0,"x1":210,"y1":390}}`, got)
}

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

// The graph takes the number of columns that draws it largest on the canvas:
// a wide short canvas lays each step out in one row, never wider than the
// widest step, and a tall narrow one stacks a step's tasks in one column.
func TestPure_TaskGraphColumnsFitTheCanvas(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskFixture+`out([P.taskFitCols(TS, 1000, 150, GEO), P.taskFitCols(TS, 110, 2000, GEO)]);`)
	assert.Equal(t, `[3,1]`, got)
}

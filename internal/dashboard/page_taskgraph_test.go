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


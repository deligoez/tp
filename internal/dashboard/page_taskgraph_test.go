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


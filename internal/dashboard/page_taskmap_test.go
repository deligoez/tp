package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// taskSteps is a spec's tasks in steps, as the Steps view groups them: step 1
// holds three tasks, step 2 two, step 3 one that waits on both of step 2.
const taskSteps = `
const mk = (id, state, deps) => ({ id, state, title: "Title " + id, depends_on: deps || [] });
const steps = [
  [mk("a", "done"), mk("b", "done"), mk("c", "ready")],
  [mk("d", "wip", ["a"]), mk("e", "waits_on_epic", ["b", "a"])],
  [mk("f", "waits_on_task", ["d", "e"])],
];
const L = P.taskMapLayout(steps, 2, (k) => "Step " + (k + 1));
`

// The Map lays the steps out as the epic map's layout reads them: a band per
// step, a step wider than the columns wrapping onto the next layer inside its
// band, one empty layer between two bands, each node's column in its row.
func TestPure_TheTaskMapPutsEachStepInItsOwnBand(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskSteps+`out({ view: L.view, layers: L.layers, columns: L.columns,
  nodes: L.nodes.map((n) => [n.id, n.layer, n.col, n.milestone]), bands: L.bands });`)
	assert.JSONEq(t, `{"view":"tasks","layers":6,"columns":2,
  "nodes":[["a",0,0,"Step 1"],["b",0,1,"Step 1"],["c",1,0,"Step 1"],["d",3,0,"Step 2"],["e",3,1,"Step 2"],["f",5,0,"Step 3"]],
  "bands":[{"milestone":"Step 1","from_layer":0,"to_layer":1},{"milestone":"Step 2","from_layer":3,"to_layer":3},{"milestone":"Step 3","from_layer":5,"to_layer":5}]}`, got)
}

// A task's mark is the epic map's word for its state, so the map draws a task
// in the colour it draws an epic in that state, and its title rides along.
func TestPure_ATaskOnTheMapCarriesTheEpicMapsMarkForItsState(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskSteps+`out(L.nodes.map((n) => [n.id, n.mark, n.title]));`)
	assert.JSONEq(t, `[["a","done","Title a"],["b","done","Title b"],["c","ready","Title c"],["d","in_progress","Title d"],["e","held","Title e"],["f","waiting","Title f"]]`, got)
}

// Of a task's dependencies, the one in the latest step, the one that set its
// step, is its tree edge; any other is a cross edge, which the map draws faint.
func TestPure_ATasksLatestDependencyIsItsTreeEdge(t *testing.T) {
	t.Parallel()
	got := runPure(t, taskSteps+`out(L.edges.map((e) => e.from + ">" + e.to + ":" + e.role));`)
	assert.JSONEq(t, `["a>d:tree","b>e:tree","a>e:cross","d>f:tree","e>f:cross"]`, got)
}

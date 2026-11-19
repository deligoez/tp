package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Tasks view is Steps or Board, and Steps is the deck.gl graph: an old
// link's tview=graph lands on Steps, and anything else not Board does too.
func TestPure_TaskViewMapsOldGraphLinksToSteps(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out(["graph", "board", "steps", null, "wires", "map"].map(P.taskView));`)
	assert.JSONEq(t, `["steps","board","steps","steps","steps","map"]`, got)
}

// One drawer shows an epic, a milestone or a task: a task only while its
// spec's Tasks view is open, an epic or a milestone only on the Epics tab, an
// epic before a milestone as drawerNext keeps them.
func TestPure_TheDrawerShowsATaskOnlyOnTheTasksView(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.drawerKind({ tab: "epics", epic: "e1", milestone: "M1" }),
  P.drawerKind({ tab: "epics", milestone: "M1" }),
  P.drawerKind({ tab: "work", view: "tasks", task: "t1", epic: "e1" }),
  P.drawerKind({ tab: "work", view: "read", task: "t1" }),
  P.drawerKind({ tab: "epics", task: "t1" }),
  P.drawerKind({ tab: "specs" }),
]);`)
	assert.JSONEq(t, `["epic","milestone","task",null,null,null]`, got)
}

// The Map draws its own legend, the epic map's, so the panel's state legend
// shows under Steps and Board only; Steps brings a task picked off its canvas
// into view, past a side drawer too, and leaves one picked on it where it is.
func TestPage_TheMapKeepsOneLegendAndStepsRevealsAnOffCanvasPick(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `(work.tview === "map" ? "" : '<div class="legend">'`)
	steps := string(served(t, "/taskgraph.js"))
	for _, s := range []string{
		"const moved = !!sel && sel !== TG.sel && !TG.picked;",
		"const hi = (S.w * s <= cw ? S.w / 2 : S.w - cw / 2 / s) + more;",
	} {
		assert.Contains(t, steps, s)
	}
}

// The Tasks view offers Steps, Map and Board, Steps drawn by the graph alone:
// no HTML steps, no wires, no side column; the summary sits in a strip above
// the panel and a task opens in the epic drawer.
func TestPage_TheTasksViewIsTheGraphUnderASummaryStrip(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `tview("steps", tk("tasks.steps")) + tview("map", tk("tasks.map")) + tview("board", tk("tasks.board"))`)
	assert.Contains(t, script, `$("sv-tasks").innerHTML = '<section class="panel tstrip"`)
	assert.Contains(t, script, `$("dbody").innerHTML = taskDetail(`)
	// The Map is the epic map's card over the spec's tasks, not a third
	// renderer.
	assert.Contains(t, script, `makeCard({ kind: "tasks", app: face,`)
	for _, gone := range []string{`tview("graph"`, `id="tside"`, "function drawWires(", "P.routeWire = function", "P.wirePath = function", `class="wires"`} {
		assert.NotContains(t, script, gone)
	}
}

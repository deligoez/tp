package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// revealShift is how far a view moves to show the span lo..hi, pad inside
// its view: not at all when it already shows, the least that brings it in
// when it does not, and the span centred when it is wider than the view.
func TestPure_RevealShiftMovesTheViewTheLeastThatShowsTheSpan(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.revealShift(100, 140, 0, 800, 16),
  P.revealShift(-60, -20, 0, 800, 16),
  P.revealShift(900, 940, 0, 800, 16),
  P.revealShift(0, 1000, 0, 800, 16),
]);`)
	assert.JSONEq(t, `[0, -76, 156, 100]`, got)
}

// A task's mark is the epic map's word for its state, so the map draws a task
// in the colour it draws an epic in that state, and its title rides along;
// a state the map has no word for reads as ready.
func TestPure_ATaskOnTheMapCarriesTheEpicMapsMarkForItsState(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const mk = (id, state) => ({ id, state, title: "Title " + id });
const M = P.taskMapMarks([[mk("a", "done"), mk("c", "ready")], [mk("d", "wip"), mk("e", "waits_on_epic")], [mk("f", "waits_on_task"), mk("g", "odd")]]);
out(Object.keys(M).sort().map((id) => [id, M[id].mark, M[id].title]));`)
	assert.JSONEq(t, `[["a","done","Title a"],["c","ready","Title c"],["d","in_progress","Title d"],["e","held","Title e"],["f","waiting","Title f"],["g","ready","Title g"]]`, got)
}

// The Map is laid out by the server, by the forest the epic map's Plan uses
// (/api/layout?view=tasks&file=), not by step in the page.
func TestPage_TheTaskMapFetchesItsForestFromTheLayoutEndpoint(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `new URLSearchParams({ view: "tasks", file })`)
	assert.NotContains(t, script, "taskMapLayout")
}

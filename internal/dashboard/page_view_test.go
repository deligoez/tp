package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPure_TheWheelZoomsWithAModifierPansInFullScreenAndElseScrollsThePage
// is 1.17.1's input rule, with no trackpad-or-mouse guess: Ctrl or ⌘ with
// the wheel, which is also how Chrome and Firefox report a pinch, zooms
// everywhere; the plain wheel, a mouse's or two fingers', pans the graph in
// full screen and scrolls the page outside it.
func TestPure_TheWheelZoomsWithAModifierPansInFullScreenAndElseScrollsThePage(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.wheelAction({ ctrlKey: true, deltaY: 3 }, false),
  P.wheelAction({ metaKey: true, deltaY: 3 }, true),
  P.wheelAction({ deltaY: 40 }, false),
  P.wheelAction({ deltaY: 40 }, true),
  P.wheelAction({ deltaX: 12 }, true),
]);`)
	assert.JSONEq(t, `["zoom", "zoom", "page", "pan", "pan"]`, got)
}


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

// TestPure_AWheelsDeltaIsReadInPixels: a wheel's delta counts pixels, lines
// of 16 pixels or pages of the canvas's height, by its deltaMode; the page
// reads every one as pixels before it zooms or pans.
func TestPure_AWheelsDeltaIsReadInPixels(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.wheelDelta({ deltaMode: 0, deltaX: 2, deltaY: -7 }, 600),
  P.wheelDelta({ deltaMode: 1, deltaX: 0, deltaY: 3 }, 600),
  P.wheelDelta({ deltaMode: 2, deltaX: 1, deltaY: -1 }, 600),
  P.wheelDelta({ deltaY: 5 }, 600),
]);`)
	assert.JSONEq(t, `[[2, -7], [0, 48], [600, -600], [0, 5]]`, got)
}

// TestPure_AZoomStepIsExponentialAndClampedPerEvent: the zoom is a power of
// two, so a step adds to it: a pinch's small deltas of 1 to 10 move it a
// little each, a mouse's 100 a full doubling, and no one event zooms by
// more than 2× either way. Safari's pinch reports a running scale, and the
// step is the log of its change since the last event, clamped the same way.
func TestPure_AZoomStepIsExponentialAndClampedPerEvent(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const r = (x) => Math.round(x * 1000) / 1000;
out([
  r(P.wheelZoom(-4)), r(P.wheelZoom(4)), P.wheelZoom(-100), P.wheelZoom(-2000), P.wheelZoom(2000),
  P.pinchZoom(1, 2), P.pinchZoom(1, 16), P.pinchZoom(2, 1), r(P.pinchZoom(1, 1.1)), P.pinchZoom(0, 1), P.pinchZoom(1, 0),
]);`)
	assert.JSONEq(t, `[0.04, -0.04, 1, 1, -1, 1, 1, -1, 0.138, 0, 0]`, got)
}


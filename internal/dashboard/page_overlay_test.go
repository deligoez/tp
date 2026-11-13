package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPage_TheDrawerIsAnOverlayOverADimmedPage is the operator's 1.17.1
// change to dashboard-ux.md §2: the drawer slides in over the page from
// the right, a dimmed backdrop behind it, and the page keeps its width;
// a click on the backdrop closes it, as Esc does; a phone keeps the bottom
// sheet, which takes its height from below.
func TestPage_TheDrawerIsAnOverlayOverADimmedPage(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `<div id="scrim" class="scrim" hidden></div>`)
	assert.Contains(t, page, ".scrim { position: fixed; inset: 0; z-index: calc(var(--z-modal) - 1);")
	assert.Contains(t, page, "@keyframes drawer-in")
	assert.NotContains(t, page, "body.dopen { padding-right: var(--drawer-w); }")
	assert.Contains(t, page, "body.dopen { padding-right: 0; padding-bottom: var(--drawer-h); }")
	for _, s := range []string{`$("scrim").hidden = !open;`, `$("scrim").addEventListener("click", () => selectEpic(null));`} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheFirstFitInFullScreenRefitsAndANoteCloses is the review's H1,
// L1 and M3: the first fit a layout opens with counts as a fit, so full
// screen refits it; the refit follows the canvas's size, read after the
// window grew; and leaving or entering full screen closes an open note,
// whose Esc the browser takes.
func TestPage_TheFirstFitInFullScreenRefitsAndANoteCloses(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{
		"G.vs = fitView();\n        G.atFit = true;", "G.atFit = false;\n          }",
		"new ResizeObserver(() => { if (G.prepared && G.deck && G.atFit) fit(); }).observe($(\"gc-canvas\"));",
		`document.addEventListener("fullscreenchange", () => { closeInfo(); `,
	} {
		assert.Contains(t, script, s)
	}
}

// TestPage_AMissedGestureEndNeverSticks is the review's L2: a gesture flag
// whose gestureend never came clears after a pause, and on the next plain
// wheel.
func TestPage_AMissedGestureEndNeverSticks(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{"const GESTURE_IDLE_MS = 500;", "G.gestureTimer = setTimeout(() => { G.gesture = false; }, GESTURE_IDLE_MS);", "if (!ev.ctrlKey && !ev.metaKey) G.gesture = false;"} {
		assert.Contains(t, script, s)
	}
}

// TestPage_TheFullScreenPillShrinksItsSearchNotItsEnds is the review's H2:
// Fit and Exit keep their place at the pill's end and never shrink; the
// search gives way down to 8rem and the mode switch shortens its words
// before them; beside the drawer the pill stays in the part of the screen
// the drawer leaves.
func TestPage_TheFullScreenPillShrinksItsSearchNotItsEnds(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	for _, s := range []string{
		"body.mapfs .gcard.fs #gc-fit, body.mapfs .gcard.fs #gc-full { order: 10; flex-shrink: 0; }",
		"body.mapfs .gcard.fs #gc-q { flex: 1 1 12rem; min-width: 8rem; }",
		"body.mapfs .gcard.fs .toolbar .seg { flex: 0 1 auto; min-width: 0; }",
		"body.mapfs .gcard.fs .toolbar .seg button { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; }",
		"body.mapfs.dopen .gcard.fs .toolbar { left: calc((100% - var(--drawer-w)) / 2); max-width: calc(100% - var(--drawer-w) - 24px); }",
	} {
		assert.Contains(t, page, s)
	}
}

// TestPage_TheReviewsSmallItemsAreInPlace is the review's L3 to L5: a KPI's
// small value keeps 30 px from its (i); the cross-edge toggle says it shows
// or hides and starts pressed; a milestone's label fits its box.
func TestPage_TheReviewsSmallItemsAreInPlace(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, ".kpi .v { padding-right: 30px; }")
	assert.Contains(t, page, `id="gc-cross" aria-pressed="true"`)
	assert.Contains(t, script, `"graph.crossTitle": "Show or hide every edge that is not a tree edge (K)"`)
	assert.Contains(t, script, "milestone: { w: 200, h: 54, col: 240, row: 110, fs: 22, chars: 14 },")
}


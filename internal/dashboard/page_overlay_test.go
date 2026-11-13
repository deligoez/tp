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


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


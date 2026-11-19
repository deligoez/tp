package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPage_ReadingTextAndTheGraphTooltipUseTheTypeScale: DESIGN.md allows
// only its type scale. The reading view's prose is body (20/32), a code
// block tiny (14/20), and the graph's hover tooltip tiny in the page's own
// faces, radius and spacing rather than deck.gl's literals.
func TestPage_ReadingTextAndTheGraphTooltipUseTheTypeScale(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, ".blk { position: relative; font: var(--type-body); overflow-wrap: anywhere; }")
	assert.Contains(t, page, ".blk pre { font: 400 14px/20px var(--mono);")
	assert.Contains(t, script, `style: { background: "rgb(" + c.ink + ")", color: "rgb(" + c.surface + ")", borderRadius: "var(--r-2)", maxWidth: "320px", font: "var(--type-tiny)", padding: "var(--sp-2) var(--sp-3)" }`)
	for _, s := range []string{`fontSize: "13px"`, "font-size:11px", `lineHeight: "1.4"`} {
		assert.NotContains(t, script, s)
	}
}

// TestPage_EveryControlNamedInReviewHasHoverAndPress: DESIGN.md asks for
// distinct hover and press states on every interactive element, laid over
// the resting fill so its colour never changes.
func TestPage_EveryControlNamedInReviewHasHoverAndPress(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	for _, s := range []string{
		".chaintext summary:hover, .dmore summary:hover { background: var(--fill-hover); }",
		".chaintext summary:active, .dmore summary:active { background: var(--fill-press); }",
		"button.pcell:hover { background-image: linear-gradient(var(--fill-hover), var(--fill-hover)); }",
		"button.pcell:active { background-image: linear-gradient(var(--fill-press), var(--fill-press)); }",
		".wsh:active { background: var(--fill-press); }",
		".gcard .legend button:active { background: var(--fill-press); }",
		".lgtoggle:active { background: var(--fill-press); }",
		".subnav button:active { background: var(--fill-press); }",
		".sec h2.sth .tc:active { background-image: linear-gradient(var(--fill-press), var(--fill-press)); }",
	} {
		assert.Contains(t, page, s)
	}
	assert.NotContains(t, page, "button.pcell:hover { background: var(--fill-press); }")
}

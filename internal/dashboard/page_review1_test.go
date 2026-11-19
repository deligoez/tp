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

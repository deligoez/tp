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

// TestPage_MonoIsForIdsAndEveryMeasureIsAToken: the mono face is kept for
// ids, paths and commands (the "tp" wordmark is the one recorded
// deviation), a measure the review named is a token, a list row is not a
// control so it has no hover, and stale data keeps its colours.
func TestPage_MonoIsForIdsAndEveryMeasureIsAToken(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	for _, s := range []string{
		".tab .c { font: var(--type-tiny); color: var(--muted);",
		".cfd text { fill: var(--muted); font: var(--type-tiny); }",
		".raxis text { fill: var(--muted); font: var(--type-tiny); }",
		"/* Deviation (DESIGN.md: one body face): the \"tp\" wordmark is set in mono, the face of the command it names. */",
		".seg { display: inline-flex; padding: var(--sp-1); gap: var(--sp-1);",
		"min-height: var(--ctl-sm); padding: 0 var(--sp-3); border: 0; border-radius: var(--r-1);",
		"padding: var(--sp-4) var(--sp-5) var(--sp-4) var(--sp-12);",
		"border-radius: var(--r-pill); background: var(--fill-strong);",
		"min-height: var(--sp-10); color: var(--muted); }",
		".matrix { border-collapse: separate; border-spacing: var(--sp-1);",
		".matrix td { min-width: 32px; height: var(--ctl-sm);",
		".stale { pointer-events: none; }",
	} {
		assert.Contains(t, page, s)
	}
	for _, s := range []string{".row:hover", "filter: grayscale(1)", "border-radius: 6px", "border-radius: 10px", "min-height: 36px", "min-height: 28px", "border-spacing: 2px", "52px"} {
		assert.NotContains(t, page, s)
	}
}

// TestPage_ATabCountIsANumberOnAPhone: below 600px a tab's count shows its
// number alone, so it never ellipsises; the words come back on wider screens.
func TestPage_ATabCountIsANumberOnAPhone(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `const tabCount = (n, words) => '<span class="cl">' + esc(words) + '</span><span class="cs">' + esc(t.num(n)) + "</span>";`)
	assert.Contains(t, script, `$("c-epics").innerHTML = g && Array.isArray(g.epics) ? tabCount(g.epics.length, t("count.epics", { n: g.epics.length }))`)
	assert.Contains(t, script, `$("c-specs").innerHTML = list && Array.isArray(list.files) ? tabCount(list.files.length, t("count.taskFiles", { n: list.files.length })) : "";`)
	page, _ := pageText(t)
	assert.Contains(t, page, ".tab .c .cl { display: none; }", "a phone hides the words")
}

// TestPage_TheTaskPanelClosesWithTheXIcon: the task panel's close button
// draws the same --ic-x mask as every other close, not a typed glyph.
func TestPage_TheTaskPanelClosesWithTheXIcon(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `<button type="button" class="popx" data-tclose aria-label="' + esc(t("task.close")) + '"></button>`)
	assert.Contains(t, page, ".popx, #dclose {")
}

package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Ready epics and Operator decisions fold as the other sections do: the
// same chevron button, a summary count while folded, the viewer's choice
// kept; both open by default, and Operator decisions opens as an alarm
// while it holds a decision.
func TestPage_ReadyAndOperatorDecisionsFold(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, id := range []string{"ready", "operator"} {
		assert.Contains(t, page, `<button type="button" class="fold" data-fold="`+id+`" aria-expanded="true" aria-controls="fold-`+id+`"><svg class="ti chev"`, id)
		assert.Contains(t, page, `<span class="fsum" id="fsum-`+id+`"></span></h2>`, id)
		assert.Contains(t, page, `<div id="fold-`+id+`" class="fbody"><div class="list" id="now-`+id+`"></div></div>`, id)
		assert.Contains(t, page, `data-info="`+id+`"`, id)
	}
	for _, s := range []string{
		`setFold("ready", { dflt: true, sum: t("fold.ready", { n: v.plan.ready.length }) });`,
		`setFold("operator", { dflt: true, alarm: queue > 0, sum: t("fold.decisions", { n: queue }) });`,
	} {
		assert.Contains(t, script, s)
	}
	for _, lang := range []string{"en", "tr"} {
		dict := dictionary(t, lang)
		assert.Contains(t, dict, "fold.ready", lang)
		assert.Contains(t, dict, "fold.decisions", lang)
	}
}

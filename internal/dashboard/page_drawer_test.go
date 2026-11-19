package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An epic's description reads first: the drawer draws it above its fields,
// Acceptance among them.
func TestPage_TheDrawerDrawsTheDescriptionAboveAcceptance(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	desc := strings.Index(script, `(String(ge.description || "").trim() ? descHTML(ge.description) : "")`)
	fields := strings.Index(script, `"<dl>" + fields.map(`)
	if assert.Positive(t, desc, "the drawer draws the description") && assert.Positive(t, fields, "the drawer draws its fields") {
		assert.Less(t, desc, fields, "the description comes before the fields, Acceptance among them")
	}
}

// An epic's Acceptance is markdown, drawn by the renderer its description
// uses: code spans, emphasis, and everything else escaped.
func TestPage_TheDrawerDrawsAcceptanceAsMarkdown(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `ge.acceptance ? [t("drawer.f.acceptance"), '<div class="ddesc">' + tpPure.markdown(ge.acceptance) + "</div>", true] : null,`)
	got := runPure(t, "out(P.markdown(\"`amount` is *rounded* <b>once</b>\"));")
	assert.JSONEq(t, `"<p><code>amount</code> is <i>rounded</i> &lt;b&gt;once&lt;/b&gt;</p>"`, got)
}

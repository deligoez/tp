package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Spec tab with nothing open opens the project's spec when it has
// exactly one; with none or several there is nothing to pick for the
// person.
func TestPure_TheSpecTabOpensTheOnlySpec(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.onlySpec([{ path: "spec/a.tasks.json" }]), P.onlySpec([{ path: "a" }, { path: "b" }]), P.onlySpec([]), P.onlySpec(undefined)]);`)
	assert.JSONEq(t, `["spec/a.tasks.json", null, null, null]`, got)
}

// With several specs the empty Spec tab names the Specs tab as a link that
// goes there, in both languages; opening the Spec tab draws it, so the only
// spec opens on the first visit.
func TestPage_TheEmptySpecTabLinksToTheSpecsTab(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, lang := range []string{"en", "tr"} {
		assert.Contains(t, string(dictionary(t, lang)["spec.none_html"]), `<button type=\"button\" class=\"link\" data-go-specs>`, lang)
	}
	for _, s := range []string{
		`if (only && state.tab === "work") {`,
		`$("work-none").innerHTML = t.html("spec.none_html");`,
		`$("work-none").addEventListener("click", (ev) => { if (ev.target.closest("[data-go-specs]")) openSpecs(); });`,
		`if (tab === "work" && !work.path) drawWork();`,
	} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, page, `data-i18n="spec.none"`)
}

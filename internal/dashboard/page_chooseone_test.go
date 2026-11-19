package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A choose-one option shows its title only when it says more than its id:
// a title that is the id, or the id's words again in other case,
// punctuation or Turkish letters, is dropped; an empty title too.
func TestPure_AnOptionTitleThatRepeatsItsIdIsDropped(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.optionTitle("l1-tetigi", "L1 Tetiği"),
  P.optionTitle("iys", "İYS"),
  P.optionTitle("ab", "ab"),
  P.optionTitle("x", ""),
  P.optionTitle("cloud-a", "Bulut sağlayıcısı A"),
]);`)
	assert.JSONEq(t, `["", "", "", "", "Bulut sağlayıcısı A"]`, got)
}

// A choose-one group's options run inline and wrap, "or" a quiet word
// between them, each title cut to a short measure with the whole title in
// its tooltip, so a group of three takes a line or two.
func TestPage_ChooseOneOptionsRunInline(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		`const ot = tpPure.optionTitle(m, titleOf(v, m));`,
		`return '<li class="opt">' + epicLink(m) + (ot ? '<span class="ot" title="' + esc(ot) + '">' + esc(ot) + "</span>" : "") + done(m, t("op.copyFor", { id: m })) + "</li>";`,
		`(i ? '<li class="or">' + esc(t("op.or")) + "</li>" : "") + option(m)).join("")`,
	} {
		assert.Contains(t, script, s)
	}
	for _, s := range []string{
		`.opts { list-style: none; margin: var(--sp-2) 0 0; padding: 0; display: flex; flex-wrap: wrap; align-items: center; gap: var(--sp-2) var(--sp-3); }`,
		`.opts .opt { display: inline-flex; align-items: center; gap: var(--sp-2); min-width: 0; max-width: 100%; font: var(--type-small); color: var(--ink); }`,
		`.opts .ot { min-width: 0; max-width: 32ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`,
	} {
		assert.Contains(t, page, s)
	}
}

package dashboard_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// i18nText is one served dictionary, as text.
func i18nText(t *testing.T, lang string) string {
	t.Helper()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/i18n/"+lang+".json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// A long now list shows its first five rows and folds the rest behind a
// "Show all N" disclosure, which stays open across the redraws a push
// brings; Ready epics and Most waited-on epics both fold this way.
func TestPage_ALongNowListFoldsPastFiveRows(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "function nowList(key, rows, empty) {")
	assert.Contains(t, script, "const [shown, rest] = tpPure.foldAt(rows, NOW_SHOWN);")
	assert.Contains(t, script, `'<details class="dmore nmore" data-dfold="' + key + '"' + (dfolds.has(key) ? " open" : "")`)
	assert.Contains(t, script, `nowList("now-ready", v.plan.ready.map(`)
	assert.Contains(t, script, `nowList("now-most", v.plan.most_waited_on.slice(0, 10).map(`)
	assert.Contains(t, i18nText(t, "en"), `"list.showAll": "Show all {n}"`)
	assert.Contains(t, i18nText(t, "tr"), `"list.showAll": "Tümünü göster ({n})"`)
}

// A choose-one decision lists its options, each with the condition it is
// (its id and title) beside its own Copy command button, "or" between them
// in plain words; each button's accessible name names its option, and the
// list keeps DESIGN.md's 8px between targets.
func TestPage_AChooseOneDecisionNamesEachOption(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `'<ul class="opts">' + c.any_of.map((m, i) => (i ? '<li class="or">' + esc(t("op.or")) + "</li>" : "") +`)
	assert.Contains(t, script, `'<li class="opt">' + epicLink(m) + '<span class="ot">' + esc(titleOf(v, m)) + "</span>" + done(m, t("op.copyFor", { id: m })) + "</li>"`)
	assert.Contains(t, script, `(label ? ' aria-label="' + esc(label) + '"' : "")`)
	assert.Contains(t, page, ".opts { list-style: none; margin: var(--sp-2) 0 0; padding: 0; display: grid; gap: var(--sp-2); }")
	assert.Contains(t, i18nText(t, "en"), `"op.copyFor": "Copy the command for {id}"`)
	assert.Contains(t, i18nText(t, "tr"), `"op.copyFor": "{id} için komutu kopyala"`)
}

// A slice row says which slice in one phrase each language orders itself
// ("K milestone'u dilimi", "Review slice for milestone K"), in the title
// column on one line, its round under it on narrow widths, never an id
// cell beside a title that a narrow column breaks mid-word.
func TestPage_ASliceRowNamesItsSliceInOnePhrase(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `sliceRow(t("op.sliceOf", { what: sliceTitle(s) }), roundText(s),`)
	assert.Contains(t, script, `const sliceRow = (label, meta, why) => '<div class="row rwide"><span class="t">' + esc(label) + '</span><span class="m">' + esc(meta) + '</span><div class="why">' + why + "</div></div>";`)
	assert.NotContains(t, script, `row(t("op.slice"), sliceTitle(s)`)
	assert.Contains(t, page, ".row.rwide { grid-template-columns: minmax(0, 1fr) auto; }")
	assert.Contains(t, page, "@container (max-width: 560px) { .row.rwide { grid-template-columns: minmax(0, 1fr); } .row.rwide .m, .row.rwide .why { grid-column: 1; } }")
	assert.Contains(t, i18nText(t, "en"), `"op.sliceOf": "Review slice for {what}"`)
	assert.Contains(t, i18nText(t, "tr"), `"op.sliceOf": "{what} dilimi"`)
}

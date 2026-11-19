package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The drawer is wide only for a stored "wide"; nothing stored, a cleared
// store or any other value keeps it narrow.
func TestPure_TheDrawerIsWideOnlyForAStoredWide(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.drawerWide("wide"), P.drawerWide("narrow"), P.drawerWide(null), P.drawerWide("1")]);`)
	assert.JSONEq(t, `[true, false, false, false]`, got)
}

// The drawer's header holds a Widen button beside Close, named in each
// language, its name turning to Narrow while the drawer is wide.
func TestPage_TheDrawerHeaderHoldsAWidenButtonBesideClose(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	wide := strings.Index(page, `<button type="button" class="key" id="dwide" aria-label="Widen"></button>`)
	closeAt := strings.Index(page, `id="dclose"`)
	if assert.Positive(t, wide, "the header holds the Widen button") {
		assert.Less(t, wide, closeAt, "Widen sits just before Close")
	}
	for lang, want := range map[string][2]string{"en": {"Widen", "Narrow"}, "tr": {"Genişlet", "Daralt"}} {
		dict := dictionary(t, lang)
		assert.JSONEq(t, `"`+want[0]+`"`, string(dict["drawer.widen"]), lang)
		assert.JSONEq(t, `"`+want[1]+`"`, string(dict["drawer.narrow"]), lang)
	}
}

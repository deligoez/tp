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

// The choice is the viewer's, kept in localStorage behind try/catch, so a
// blocked store still draws the page; toggling it neither redraws the drawer
// nor moves focus, and Escape still closes the drawer.
func TestPage_TheWideChoiceIsRememberedAndKeepsFocusAndEscape(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{
		`try { return localStorage.getItem(WIDE_KEY); } catch (e) { return null; }`,
		`try { localStorage.setItem(WIDE_KEY, on ? "wide" : "narrow"); } catch (e) { /* the choice lasts this load */ }`,
		`$("dwide").addEventListener("click", () => setWide(!wide, true));`,
		`if (ev.key === "Escape" && !viewMenuOpen() && !$("drawer").hidden && $("pop").hidden) selectEpic(null);`,
	} {
		assert.Contains(t, script, s)
	}
	start := strings.Index(script, "function setWide(")
	if assert.Positive(t, start) {
		body := script[start : start+strings.Index(script[start:], "\n  }\n")]
		for _, s := range []string{"drawDrawer(", "selectEpic(", ".focus("} {
			assert.NotContains(t, body, s, "toggling the width neither redraws nor moves focus")
		}
	}
}

// A wide drawer takes most of the viewport and keeps its prose to a
// readable measure in the body type: a main column of 40rem beside the
// lists; a narrow viewport has no Widen button, the drawer being its width.
func TestPage_AWideDrawerKeepsItsProseToAReadableMeasure(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		`.drawer.wide { width: min(1200px, calc(100vw - var(--sp-16))); }`,
		`.drawer.wide .dbody { grid-template-columns: minmax(0, 40rem) minmax(0, 1fr); column-gap: var(--sp-12); }`,
		`.drawer.wide .ddesc { font: var(--type-body); }`,
		`@media (max-width: 900px) { #dwide { display: none; } .drawer.wide { width: var(--drawer-w); } .drawer.wide .dbody { grid-template-columns: minmax(0, 1fr); } }`,
	} {
		assert.Contains(t, page, s)
	}
	assert.Contains(t, script, `'<div class="dmain">'`)
	assert.Contains(t, script, `'<div class="dside">'`)
}

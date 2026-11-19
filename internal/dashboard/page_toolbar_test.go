package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPage_TheGraphToolbarCarriesNoDuplicateFitOrKeys is the simplify
// note's pass 2: the canvas's zoom group already holds Fit and the open
// legend already lists the keys, so the toolbar repeats neither. F still
// fits, from the canvas and from the page.
func TestPage_TheGraphToolbarCarriesNoDuplicateFitOrKeys(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{`id="gc-fit"`, `data-info="keys"`, `gkeys`, `"graph.keysLabel"`, `"info.keys"`} {
		assert.NotContains(t, page+script, s)
	}
	assert.Contains(t, page, `id="gc-zfit"`)
	assert.Contains(t, script, `} else if (ev.key === "f" || ev.key === "F") fit();`)
}

// TestPage_TheViewMenuHoldsTheOwnerColourAndCrossControls: one View
// button opens a popover (the browser's top layer, so the full-screen
// pill's overflow never clips it; Esc and a click outside close it) that
// holds the owner picker, Colour by owner and Cross edges, in that order.
func TestPage_TheViewMenuHoldsTheOwnerColourAndCrossControls(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `<button type="button" class="key vbtn" id="gc-viewbtn" popovertarget="gc-viewpop" aria-haspopup="true" aria-controls="gc-viewpop" aria-expanded="false">`)
	open := strings.Index(page, `<div class="vpop" id="gc-viewpop" popover="auto" role="group" aria-labelledby="gc-viewbtn">`)
	require.GreaterOrEqual(t, open, 0, "the popover exists")
	shut := open + strings.Index(page[open:], "</div>")
	last := open
	for _, id := range []string{`id="gc-owner-pick"`, `id="gc-ocolour"`, `id="gc-cross"`} {
		at := strings.Index(page, id)
		assert.Greater(t, at, last, "%s sits in the popover, after the one before", id)
		assert.Less(t, at, shut, "%s sits in the popover", id)
		last = at
	}
	for _, s := range []string{`$("gc-viewpop").addEventListener("toggle"`, `$("gc-viewbtn").setAttribute("aria-expanded", String(open));`, `pure.viewChanged(G, owned)`} {
		assert.Contains(t, script, s)
	}
	assert.NotContains(t, page, "body.mapfs .gcard.fs .toolbar.tight #gc-cross", "the popover keeps its words whole")
	assert.Contains(t, page, "body.mapfs.dopen .gcard.fs .gcountfs, body.mapfs.dopen .gcard.fs .toolbar > .gpick > span { display: none; }",
		"only the pill's own pickers drop their words; the popover's owner label stays")
}

// TestPure_TheViewButtonCountsTheSettingsAwayFromTheirDefault: the badge
// on View counts an owner chosen, colour by owner on and cross edges
// hidden; the owner settings count only while the graph has owners.
func TestPure_TheViewButtonCountsTheSettingsAwayFromTheirDefault(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.viewChanged({}, true), P.viewChanged({ ownerSel: "ana" }, true), P.viewChanged({ ownerSel: null }, true),
  P.viewChanged({ ownerColour: true, showCross: false }, true), P.viewChanged({ ownerSel: "ana", ownerColour: true, showCross: false }, true),
  P.viewChanged({ ownerSel: "ana", ownerColour: true, showCross: false }, false),
]);`)
	assert.JSONEq(t, `[0, 1, 1, 2, 3, 1]`, got)
}

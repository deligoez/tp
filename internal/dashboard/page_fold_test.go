package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPure_AFoldOpensOnItsDefaultUntilTheViewerChooses: a fold nobody has
// touched follows its default, a stored choice from an earlier visit
// overrides the default, and a press on this page overrides both.
func TestPure_AFoldOpensOnItsDefaultUntilTheViewerChooses(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.foldOpen({ dflt: false }),
  P.foldOpen({ dflt: true }),
  P.foldOpen({ dflt: false, stored: "open" }),
  P.foldOpen({ dflt: true, stored: "closed" }),
  P.foldOpen({ dflt: false, stored: "open", touched: false }),
  P.foldOpen({ dflt: true, stored: "closed", touched: true }),
  P.foldOpen({ dflt: false, stored: "junk" }),
]);`)
	assert.JSONEq(t, `[false, true, true, false, false, true, false]`, got)
}

// TestPure_AnAlarmOpensAFoldUntilTheViewerClosesIt: a fold whose content
// carries an alarm (a held epic, a lint error) opens even over a stored
// "closed", so an alarm is never folded away by an earlier visit; a press
// on this page still closes it.
func TestPure_AnAlarmOpensAFoldUntilTheViewerClosesIt(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.foldOpen({ dflt: false, alarm: true }),
  P.foldOpen({ dflt: false, stored: "closed", alarm: true }),
  P.foldOpen({ dflt: false, alarm: true, touched: false }),
]);`)
	assert.JSONEq(t, `[true, true, false]`, got)
}

// TestPage_EveryFoldIsAButtonThatNamesWhatItControls: each fold's heading
// is a button carrying aria-expanded and aria-controls naming its body, so
// the folded state is announced and the body found; the body is the card's
// content, hidden while folded.
func TestPage_EveryFoldIsAButtonThatNamesWhatItControls(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	for _, id := range []string{"waiting", "band", "cfd", "matrix", "slices", "kpimore"} {
		assert.Contains(t, page, `data-fold="`+id+`" aria-expanded="false" aria-controls="fold-`+id+`"`, "fold %s", id)
		assert.Contains(t, page, `id="fold-`+id+`" class="fbody" hidden`, "fold %s", id)
	}
	assert.Equal(t, 6, strings.Count(page, `class="fold"`), "six folds")
}

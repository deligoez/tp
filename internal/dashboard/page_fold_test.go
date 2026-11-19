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

// TestPure_AnAlarmThatArrivesAfterAPressReopensTheFold: a press remembers
// whether an alarm was showing when it was made. An alarm that arrives
// later (a live push brings a held epic) reopens a fold the viewer closed
// while all was quiet; an alarm the viewer already closed stays closed, and
// a press with no alarm showing keeps its own choice.
func TestPure_AnAlarmThatArrivesAfterAPressReopensTheFold(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.foldOpen({ dflt: false, alarm: true, touched: { open: false, alarm: false } }),
  P.foldOpen({ dflt: false, alarm: true, touched: { open: false, alarm: true } }),
  P.foldOpen({ dflt: false, alarm: false, touched: { open: true, alarm: false } }),
  P.foldOpen({ dflt: true, alarm: false, touched: { open: false, alarm: true } }),
]);`)
	assert.JSONEq(t, `[true, false, true, false]`, got)
}

// TestPure_AnAlarmThatClearsForgetsThePressMadeDuringIt: a press made while
// an alarm showed is dropped once that alarm clears, so an alarm that comes
// back later in the same load counts as new and reopens the fold; a press
// made while quiet, or one whose alarm still shows, is kept.
func TestPure_AnAlarmThatClearsForgetsThePressMadeDuringIt(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const during = { open: false, alarm: true };
const kept = P.foldPress(during, true);
const cleared = P.foldPress(during, false);
out([
  kept === during,
  cleared === undefined,
  P.foldPress({ open: false, alarm: false }, false).open === false,
  P.foldPress(undefined, true) === undefined,
  P.foldOpen({ dflt: false, stored: "closed", alarm: true, touched: P.foldPress(cleared, true) }),
]);`)
	assert.JSONEq(t, `[true, true, true, true, true]`, got)
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
	assert.Equal(t, 8, strings.Count(page, `class="fold"`), "six folds drawn closed and the two now lists drawn open")
}

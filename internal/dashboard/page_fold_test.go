package dashboard_test

import (
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

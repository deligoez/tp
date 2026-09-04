package cli_test

import (
	"testing"
)

// TestReviewRecordRolelessRowsAdviseOnce: the role-less-row advisory fired once
// per ROW on raw os.Stderr. Two costs, both real in the loop: it ignored
// --quiet, and an N-row findings file paid N near-identical lines. One
// condition costs one Notice line carrying the count. tp audit's --record was
// fixed first; review carried the same defect verbatim, and both now share one
// rolelessRows tracker so the wording cannot drift apart per phase — which is
// why both phases run the same assertion body.
func TestReviewRecordRolelessRowsAdviseOnce(t *testing.T) {
	t.Parallel()
	assertRolelessRowsAdviseOnce(t, "review",
		`{"severity":"low","category":"consistency","location":"L1","finding":"f","suggestion":"s"}`,
		"findings")
}

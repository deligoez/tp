package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A blocker stops the step it is about (internal/engine/resumeblockers.go):
// no-ready-task and epic-blocked stop implement, the review loop's codes
// review, audit's audit, and spec-stale, which tp resume raises in implement
// and audit, is about the review loop, so it never pauses implement. A code
// the page does not know stops the step it stands in. spec-stale at the
// review cap, the operator's raise-review-cap, reads as its own text; its
// commands stay the operator's.
func TestPure_ABlockerStopsOnlyTheStepItIsAbout(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const stale = { class: "escalate", code: "spec-stale", data: { spec: "s.md",
  operator: { decision: "raise-review-cap", commands: ["tp review s.md", "tp review s.md --record-inbox"] } } };
out([
  P.blockerStep("spec-stale", "implement"), P.blockerStep("spec-stale", "audit"),
  P.blockerStep("no-ready-task", "implement"), P.blockerStep("epic-blocked", "implement"),
  P.blockerStep("review-budget-exhausted", "review"), P.blockerStep("review-carried-blocking", "review"),
  P.blockerStep("audit-budget-exhausted", "audit"), P.blockerStep("a-new-code", "implement"),
  P.blockerKey(stale), P.blockerKey({ code: "spec-stale", data: { commands: ["tp review s.md"] } }),
  P.blockerKey({ code: "no-ready-task", data: {} }), P.blockerCommands(stale),
]);`)
	assert.JSONEq(t, `["review", "review", "implement", "implement", "review", "review", "audit", "implement",
		"spec-stale-at-cap", "spec-stale", "no-ready-task", ["tp review s.md", "tp review s.md --record-inbox"]]`, got)
}

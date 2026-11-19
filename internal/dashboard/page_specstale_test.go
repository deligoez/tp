package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// The Spec view pauses the current step only for a blocker that stops it;
// one about another step, as spec-stale at the cap is about review while
// implement runs, rings that step gold and shows as a warning note naming
// it, in words that say implementation can continue.
func TestPage_SpecStaleAtTheCapWarnsOnReviewAndLeavesImplementRunning(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{
		`const held = new Set(escalations(d).map((b) => tpPure.blockerStep(b.code, phase)));`,
		`const paused = st === "cur" && held.has(p);`,
		`const warn = st !== "cur" && held.has(p);`,
		`(paused ? " op" : warn ? " warn" : "")`,
		`escalations(d).map((b) => opNote(b, d.resume.phase)).join("")`,
		`'<div class="opnote warn"><span class="mk">' + WARN + "</span><div><strong>" + esc(word(STEP_LABEL, step)) + "</strong> " + blockerHTML(b) + "</div></div>"`,
		`tpPure.blockerText(tpPure.blockerKey(b), BLOCKER_TEXT, t)`,
		`"spec-stale-at-cap": tk("blocker.code.spec-stale-at-cap")`,
	} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, `.stepper li.warn { box-shadow: inset 0 0 0 2px var(--fill-warning-strong); }`)
	assert.NotContains(t, script, `const paused = escalations(d).length > 0;`, "an escalation alone no longer pauses the current step")
	words := map[string]string{"en": "Implementation can continue", "tr": "Uygulama sürebilir"}
	for lang, want := range words {
		raw, ok := dictionary(t, lang)["blocker.code.spec-stale-at-cap"]
		require.True(t, ok, lang)
		assert.True(t, strings.Contains(string(raw), want), "%s: %s", lang, raw)
	}
}

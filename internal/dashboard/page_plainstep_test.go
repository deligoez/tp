package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A blocker's code reads as plain words in the page's language, an unknown
// code as a generic "waits on the operator"; the code itself is never the
// text a person reads.
func TestPure_ABlockerCodeReadsAsPlainWords(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const table = { "spec-stale": "blocker.code.spec-stale" };
out([P.blockerText("spec-stale", table, T), P.blockerText("no-such-code", table, T)]);`)
	assert.JSONEq(t, `["The spec changed since its last review; it needs another review round.", "Waits on the operator's decision."]`, got)
}

// Every blocker code the engine emits has its words in both languages.
func TestPage_EveryBlockerCodeHasItsWords(t *testing.T) {
	t.Parallel()
	codes := []string{"spec-stale", "review-budget-exhausted", "audit-budget-exhausted", "review-carried-blocking", "epic-blocked", "no-ready-task", "unexplained-changes", "other"}
	for _, lang := range []string{"en", "tr"} {
		dict := dictionary(t, lang)
		for _, c := range codes {
			if assert.Contains(t, dict, "blocker.code."+c, lang) {
				assert.NotRegexp(t, tpCommand, string(dict["blocker.code."+c]), "%s %s", lang, c)
			}
		}
	}
}

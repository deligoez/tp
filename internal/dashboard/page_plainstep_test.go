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

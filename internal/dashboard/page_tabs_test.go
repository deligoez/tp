package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPure_ThePathTailIsTheFileName: the Spec tab's short label is the
// open spec's file name, so a phone-width tab row still names it.
func TestPure_ThePathTailIsTheFileName(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.pathTail("spec/backlog/migration.md"), P.pathTail("top.md"), P.pathTail("")])`)
	assert.JSONEq(t, `["migration.md","top.md",""]`, got)
}

// TestPage_TheSpecTabNamesItsFileAtPhoneWidth: below 600px the Spec tab
// shows only the file name, ellipsised inside its own third of the row,
// the full path still in its tooltip; the subtitles hide, the counts stay.
func TestPage_TheSpecTabNamesItsFileAtPhoneWidth(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, `'<span class="cl">' + esc(work.path || "") + '</span><span class="cs">' + esc(tpPure.pathTail(work.path || "")) + "</span>"`)
	assert.Contains(t, script, `$("tab-work").title = work.path || "";`)
	css := string(served(t, "/style.css"))
	assert.Contains(t, css, `.tab .c .cs { display: none; }`)
	assert.Contains(t, css, `@media (max-width: 599px) { .tab { flex: 1 1 0; min-width: 0; flex-direction: column; gap: 0; padding: var(--sp-2) var(--sp-3); } .tab > span:not(.c), .tab .c .cl { display: none; } .tab .c { max-width: 100%; } .tab .c .cs { display: inline; } }`)
}

package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// More figures sits directly under the KPI row, a small disclosure that
// counts its tiles and opens them in place, before the now lists; its
// state is the viewer's, as every fold's is.
func TestPage_MoreFiguresOpensUnderTheKPIRow(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	kpis := strings.Index(page, `id="kpis"`)
	more := strings.Index(page, `id="kpimore"`)
	now := strings.Index(page, `id="now"`)
	if assert.Positive(t, kpis) && assert.Positive(t, more) {
		assert.Less(t, kpis, more, "More figures follows the KPI row")
		assert.Less(t, more, now, "and comes before the now lists")
	}
	assert.Contains(t, page, `<span data-i18n="kpi.more">More figures</span> <span class="num muted" id="kpimore-n"></span></button>`)
	assert.Contains(t, page, `.kpimore > h2.fh { margin: 0; font: var(--type-label); }`)
	assert.Contains(t, script, `$("kpimore-n").textContent = "(" + t.num(more.length) + ")";`)
}

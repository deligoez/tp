package dashboard_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// i18nText is one served dictionary, as text.
func i18nText(t *testing.T, lang string) string {
	t.Helper()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/i18n/"+lang+".json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// A long now list shows its first five rows and folds the rest behind a
// "Show all N" disclosure, which stays open across the redraws a push
// brings; Ready epics and Most waited-on epics both fold this way.
func TestPage_ALongNowListFoldsPastFiveRows(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "function nowList(key, rows, empty) {")
	assert.Contains(t, script, "const [shown, rest] = tpPure.foldAt(rows, NOW_SHOWN);")
	assert.Contains(t, script, `'<details class="dmore nmore" data-dfold="' + key + '"' + (dfolds.has(key) ? " open" : "")`)
	assert.Contains(t, script, `nowList("now-ready", v.plan.ready.map(`)
	assert.Contains(t, script, `nowList("now-most", v.plan.most_waited_on.slice(0, 10).map(`)
	assert.Contains(t, i18nText(t, "en"), `"list.showAll": "Show all {n}"`)
	assert.Contains(t, i18nText(t, "tr"), `"list.showAll": "Tümünü göster ({n})"`)
}

package dashboard

import (
	"io"
	"net/http"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandler_StampsThePagesFilesSoAnUpgradeNeverMixesThem: the page asks
// for its scripts, its styles and its words with ?v=<stamp>, a hash of the
// embedded files, and is itself never cached, so a browser that kept an
// older tp's files never runs an old pure.js beside a new app.js; a
// stamped file is cached as immutable, an unstamped one revalidated.
func TestHandler_StampsThePagesFilesSoAnUpgradeNeverMixesThem(t *testing.T) {
	t.Parallel()
	resp := getWith("/", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	m := regexp.MustCompile(`<html lang="en" data-v="([0-9a-f]{12})">`).FindStringSubmatch(string(body))
	require.NotNil(t, m, "the page carries its stamp")
	stamp := m[1]
	for _, ref := range []string{`src="pure.js?v=`, `src="app.js?v=`, `src="graph.js?v=`, `href="style.css?v=`} {
		assert.Contains(t, string(body), ref+stamp+`"`)
	}
	assert.Equal(t, "public, max-age=31536000, immutable", getWith("/app.js?v="+stamp, nil).Header.Get("Cache-Control"))
	assert.Equal(t, "no-cache", getWith("/app.js", nil).Header.Get("Cache-Control"))
	assert.Equal(t, "no-cache", getWith("/app.js?v=old", nil).Header.Get("Cache-Control"))
	assert.Contains(t, string(readPage(t, "app.js")), `".json?v=" + document.documentElement.dataset.v`, "the words are fetched stamped too")
	assert.NotEqual(t, stampOf(fstest.MapFS{"a.js": {Data: []byte("1")}}), stampOf(fstest.MapFS{"a.js": {Data: []byte("2")}}))
}

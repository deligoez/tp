package dashboard_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/dashboard"
)

const testPort = 7377

// serve sends one request through the handler bound to testPort.
func serve(t *testing.T, method, host, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, "http://"+host+path, http.NoBody)
	req.Host = host
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	dashboard.Handler(testPort, nil).ServeHTTP(rec, req)
	return rec.Result()
}

// assertNoCORS fails when a response carries any CORS header.
func assertNoCORS(t *testing.T, resp *http.Response) {
	t.Helper()
	for name := range resp.Header {
		assert.False(t, strings.HasPrefix(strings.ToLower(name), "access-control-"), "response carries CORS header %s", name)
	}
}

// TestHandler_RefusesEveryMethodButGET is §1's method rule: GET serves, and
// every other method is refused before anything is served.
func TestHandler_RefusesEveryMethodButGET(t *testing.T) {
	t.Parallel()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assertNoCORS(t, resp)
	for _, m := range []string{http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		resp := serve(t, m, "127.0.0.1:7377", "/")
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, m)
		assertNoCORS(t, resp)
	}
}

// TestHandler_RefusesAForeignHost is §1's Host rule: the bound address and
// port, or localhost with that port, serve; any other Host is refused, which
// is what closes DNS rebinding.
func TestHandler_RefusesAForeignHost(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"127.0.0.1:7377", "localhost:7377", "LOCALHOST:7377"} {
		assert.Equal(t, http.StatusOK, serve(t, http.MethodGet, host, "/").StatusCode, host)
	}
	for _, host := range []string{"evil.example:7377", "127.0.0.1:7378", "localhost:8080", "127.0.0.1", "localhost", "0.0.0.0:7377", "[::1]:7377"} {
		resp := serve(t, http.MethodGet, host, "/")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, host)
		assertNoCORS(t, resp)
	}
}

// TestHandler_ServesThePageScriptAndStylesFromTheEmbed is §1's embed rule:
// the page links its script and styles, and each is served from the binary.
func TestHandler_ServesThePageScriptAndStylesFromTheEmbed(t *testing.T) {
	t.Parallel()
	page := serve(t, http.MethodGet, "127.0.0.1:7377", "/")
	body, err := io.ReadAll(page.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `src="app.js"`)
	assert.Contains(t, string(body), `href="style.css"`)

	for path, kind := range map[string]string{"/app.js": "javascript", "/style.css": "text/css"} {
		resp := serve(t, http.MethodGet, "127.0.0.1:7377", path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		assert.Contains(t, resp.Header.Get("Content-Type"), kind, path)
	}
}

// pageText is the page, its script and its styles as the handler serves them.
func pageText(t *testing.T) (page, script string) {
	t.Helper()
	var all strings.Builder
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		resp := serve(t, http.MethodGet, "127.0.0.1:7377", path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		all.Write(body)
		if path == "/app.js" {
			script = string(body)
		}
	}
	return all.String(), script
}

// TestPage_ListensToTheEventStreamAndLoadsNothingRemote is §2 and §3 at the
// page: its script subscribes to the event stream and to each of the three
// payloads by its event name, and neither the page, the script nor the
// styles reference a resource over http or https, so the page needs no
// network beside the dashboard it came from.
func TestPage_ListensToTheEventStreamAndLoadsNothingRemote(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, name := range []string{`"/api/events"`, `"epic-export"`, `"export-list"`, `"summary"`} {
		assert.Contains(t, script, name)
	}
	assert.NotContains(t, page, "http://")
	assert.NotContains(t, page, "https://")
}

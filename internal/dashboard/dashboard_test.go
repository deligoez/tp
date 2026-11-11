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
	dashboard.Handler(testPort).ServeHTTP(rec, req)
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


package cli_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dashboardError is tp's error as a dashboard GET answers it.
type dashboardError struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
	Hint  string `json:"hint"`
}

// getDashboardError fetches path from srv, requires status, and decodes the
// error body.
func getDashboardError(t *testing.T, srv *dashboardServer, path string, status int) dashboardError {
	t.Helper()
	got, body := getPage(t, strings.TrimSuffix(srv.url, "/")+path)
	require.Equal(t, status, got, "GET %s: %s", path, body)
	var e dashboardError
	require.NoError(t, json.Unmarshal([]byte(body), &e), "GET %s: %s", path, body)
	assert.NotEmpty(t, e.Error, path)
	assert.NotEmpty(t, e.Hint, path)
	return e
}

// TestDashboard_GetAnswersTpsErrorAndRefusesOutsideTheRepo is §2's GET on a
// bad request: an unknown file or slice answers tp's error, code and hint, a
// malformed query is a usage error, and a task file outside the repository —
// named by a relative path, an absolute one or a symlink inside — is refused.
func TestDashboard_GetAnswersTpsErrorAndRefusesOutsideTheRepo(t *testing.T) {
	t.Parallel()
	dir, _ := twoRoundSlice(t)
	writeTaskFileRaw(t, dir, `[]`)
	outside := filepath.Join(t.TempDir(), "outside.tasks.json")
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outside, data, 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link.tasks.json")))
	rel, err := filepath.Rel(dir, outside)
	require.NoError(t, err)
	srv := startDashboard(t, dir, freePort(t))

	assert.Equal(t, 3, getDashboardError(t, srv, "/api/export?file=missing.tasks.json", http.StatusNotFound).Code)
	assert.Equal(t, 4, getDashboardError(t, srv, "/api/history?milestone=Z", http.StatusNotFound).Code)
	assert.Equal(t, 3, getDashboardError(t, srv, "/api/history?epic=a", http.StatusNotFound).Code)
	for _, bad := range []string{"/api/history", "/api/history?milestone=A&epic=a", "/api/history?milestone=A&from=1", "/api/history?milestone=A&name=n", "/api/history?milestone=A&from=x&to=2", "/api/epic-history"} {
		assert.Equal(t, 2, getDashboardError(t, srv, bad, http.StatusBadRequest).Code, bad)
	}
	for _, file := range []string{rel, outside, "link.tasks.json"} {
		e := getDashboardError(t, srv, "/api/export?file="+url.QueryEscape(file), http.StatusForbidden)
		assert.Equal(t, 2, e.Code, file)
		assert.Contains(t, e.Error, "outside", file)
	}
}

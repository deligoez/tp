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
// malformed query is a usage error, and a task file outside the repository,
// by a relative path or an absolute one, is not listed and so not served.
func TestDashboard_GetAnswersTpsErrorAndRefusesOutsideTheRepo(t *testing.T) {
	t.Parallel()
	dir, _ := twoRoundSlice(t)
	writeTaskFileRaw(t, dir, `[]`)
	outside := filepath.Join(t.TempDir(), "outside.tasks.json")
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outside, data, 0o600))
	rel, err := filepath.Rel(dir, outside)
	require.NoError(t, err)
	srv := startDashboard(t, dir, freePort(t))

	assert.Equal(t, 4, getDashboardError(t, srv, "/api/history?milestone=Z", http.StatusNotFound).Code)
	assert.Equal(t, 3, getDashboardError(t, srv, "/api/history?epic=a", http.StatusNotFound).Code)
	for _, bad := range []string{"/api/history", "/api/history?milestone=A&epic=a", "/api/history?milestone=A&from=1", "/api/history?milestone=A&name=n", "/api/history?milestone=A&from=x&to=2", "/api/epic-history"} {
		assert.Equal(t, 2, getDashboardError(t, srv, bad, http.StatusBadRequest).Code, bad)
	}
	for _, file := range []string{"missing.tasks.json", rel, outside} {
		requireNotListed(t, srv, file, "\x00")
	}
}

// requireSameErrorAsCommand requires GET /api/export?file=<file> to answer
// 404 with the error, code and hint tp export --file <file> exits with in
// dir.
func requireSameErrorAsCommand(t *testing.T, srv *dashboardServer, dir, file string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "export", "--file", file)
	require.NotEqual(t, 0, code, file)
	var want dashboardError
	require.NoError(t, json.Unmarshal([]byte(stderr), &want), "tp export --file %s: %s", file, stderr)
	got := getDashboardError(t, srv, "/api/export?file="+url.QueryEscape(file), http.StatusNotFound)
	assert.Equal(t, want, got, file)
}

// TestDashboard_DanglingInRepoSymlinkIsNotFound: a listed task file that is
// a symlink to nothing answers tp export --file's own not-found error.
func TestDashboard_DanglingInRepoSymlinkIsNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	require.NoError(t, os.Symlink("gone.tasks.json", filepath.Join(dir, "dangling.tasks.json")))
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	requireSameErrorAsCommand(t, srv, dir, "dangling.tasks.json")
}

// TestDashboard_AbsoluteSymlinkInsideTheRepoIsServedOnlyUnderItsListedPath:
// a task file that is a symlink with an absolute target inside the
// repository is listed under the path tp export --list prints for it, and
// that path answers byte-equal to tp export --file; the link's own name, not
// listed, answers 404 with code 4.
func TestDashboard_AbsoluteSymlinkInsideTheRepoIsServedOnlyUnderItsListedPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	require.NoError(t, os.Symlink(filepath.Join(dir, "spec.tasks.json"), filepath.Join(dir, "abs.tasks.json")))
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	paths := listedPaths(t, dir, nil)
	require.NotContains(t, paths, "abs.tasks.json")
	for _, p := range paths {
		requireSameAsCommand(t, srv, dir, "/api/export?file="+url.QueryEscape(p), "export", "--file", p)
	}
	requireNotListed(t, srv, "abs.tasks.json", "\x00")
}

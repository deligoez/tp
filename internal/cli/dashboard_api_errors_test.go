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

// TestDashboard_RefusesAMissingPathThroughASymlinkOutsideTheRepo: a task
// file named through an in-repo symlink to a directory outside the
// repository is refused 403 whether the rest of the path exists or not, so
// the answer does not tell a caller which outside directories exist; a
// missing path inside the repository still reaches tp's not-found error.
func TestDashboard_RefusesAMissingPathThroughASymlinkOutsideTheRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outside, "realdir"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "lnk")))
	srv := startDashboard(t, dir, freePort(t))

	for _, file := range []string{"lnk/realdir/x.tasks.json", "lnk/nope/y.tasks.json", "lnk/nope/deeper/z.tasks.json"} {
		e := getDashboardError(t, srv, "/api/export?file="+url.QueryEscape(file), http.StatusForbidden)
		assert.Contains(t, e.Error, "outside", file)
	}
	assert.Equal(t, 3, getDashboardError(t, srv, "/api/export?file="+url.QueryEscape("nope/deeper/z.tasks.json"), http.StatusNotFound).Code)
}

// TestDashboard_RefusesADotDotPastAnInRepoSymlinkLeadingOut: a path that
// climbs out of an in-repo symlink with .. names, as the OS opens it, the
// parent of the symlink's target, not the repository: lnk/../evil.tasks.json
// with lnk pointing at a directory outside is refused 403, existing or not,
// while the same climb out of a real in-repo directory still answers.
func TestDashboard_RefusesADotDotPastAnInRepoSymlinkLeadingOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outside, "inner"), 0o755))
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(outside, "evil.tasks.json"), data, 0o600))
	require.NoError(t, os.Symlink(filepath.Join(outside, "inner"), filepath.Join(dir, "lnk")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	srv := startDashboard(t, dir, freePort(t))

	for _, file := range []string{"lnk/../evil.tasks.json", "lnk/../nope.tasks.json", "lnk/../nope/z.tasks.json", "sub/../lnk/../evil.tasks.json"} {
		e := getDashboardError(t, srv, "/api/export?file="+url.QueryEscape(file), http.StatusForbidden)
		assert.Contains(t, e.Error, "outside", file)
	}
	got, body := getPage(t, strings.TrimSuffix(srv.url, "/")+"/api/export?file="+url.QueryEscape("sub/../spec.tasks.json"))
	assert.Equal(t, http.StatusOK, got, body)
}

// requireSameErrorAsCommand requires GET /api/export?file=<file> to answer
// status with the error, code and hint tp export --file <file> exits with in
// dir.
func requireSameErrorAsCommand(t *testing.T, srv *dashboardServer, dir, file string, status int) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "export", "--file", file)
	require.NotEqual(t, 0, code, file)
	var want dashboardError
	require.NoError(t, json.Unmarshal([]byte(stderr), &want), "tp export --file %s: %s", file, stderr)
	got := getDashboardError(t, srv, "/api/export?file="+url.QueryEscape(file), status)
	assert.Equal(t, want, got, file)
}

// TestDashboard_MissingTaskFileNamesThePathAsGiven: a task file that does
// not exist answers 404 with tp export's own error, naming the path as the
// caller gave it, not a resolved absolute one.
func TestDashboard_MissingTaskFileNamesThePathAsGiven(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	requireSameErrorAsCommand(t, srv, dir, "missing.tasks.json", http.StatusNotFound)
}

// TestDashboard_DotDotThroughAMissingDirectoryAnswersAsTheCommand:
// nope/../spec.tasks.json stays inside the repository, and since the OS
// cannot open a path through a directory that does not exist, it answers
// tp export's not-found error, not the outside-the-repository refusal.
func TestDashboard_DotDotThroughAMissingDirectoryAnswersAsTheCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	requireSameErrorAsCommand(t, srv, dir, "nope/../spec.tasks.json", http.StatusNotFound)
}

// TestDashboard_DanglingInRepoSymlinkIsNotFound: an in-repo symlink whose
// in-repo target does not exist names nothing outside the repository and
// answers tp export's not-found error.
func TestDashboard_DanglingInRepoSymlinkIsNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	require.NoError(t, os.Symlink("gone.tasks.json", filepath.Join(dir, "dangling.tasks.json")))
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	requireSameErrorAsCommand(t, srv, dir, "dangling.tasks.json", http.StatusNotFound)
}

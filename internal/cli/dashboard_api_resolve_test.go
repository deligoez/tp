package cli_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// linkChain makes n in-repo relative symlinks in dir, link1.tasks.json ->
// link2.tasks.json -> ... -> linkN.tasks.json, the last pointing at target,
// and returns the first one's name.
func linkChain(t *testing.T, dir string, n int, target string) string {
	t.Helper()
	name := func(i int) string { return "link" + strconv.Itoa(i) + ".tasks.json" }
	for i := 1; i < n; i++ {
		require.NoError(t, os.Symlink(name(i+1), filepath.Join(dir, name(i))))
	}
	require.NoError(t, os.Symlink(target, filepath.Join(dir, name(n))))
	return name(1)
}

// requireRefusedAsOutside requires GET /api/export?file=<file> to answer 403,
// code 2, the outside-the-repository refusal.
func requireRefusedAsOutside(t *testing.T, srv *dashboardServer, file string) {
	t.Helper()
	e := getDashboardError(t, srv, "/api/export?file="+url.QueryEscape(file), http.StatusForbidden)
	assert.Equal(t, 2, e.Code, file)
	assert.Contains(t, e.Error, "outside", file)
}

// TestDashboard_RefusesALongInRepoSymlinkChainLeadingOut: a chain of nine
// in-repo symlinks, one past os.Root's limit of eight, whose last link points
// at a task file outside the repository is refused, not served by a read that
// follows the chain as the OS does.
func TestDashboard_RefusesALongInRepoSymlinkChainLeadingOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	outside := filepath.Join(t.TempDir(), "outside.tasks.json")
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outside, data, 0o600))
	first := linkChain(t, dir, 9, outside)
	srv := startDashboard(t, dir, freePort(t))

	requireRefusedAsOutside(t, srv, first)
}

// TestDashboard_RefusesASymlinkChainPastTheOSLimit: a chain of 33 in-repo
// symlinks, past the OS's own limit, whose last link points outside the
// repository is refused as well.
func TestDashboard_RefusesASymlinkChainPastTheOSLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	outside := filepath.Join(t.TempDir(), "outside.tasks.json")
	require.NoError(t, os.WriteFile(outside, []byte(`[]`), 0o600))
	first := linkChain(t, dir, 33, outside)
	srv := startDashboard(t, dir, freePort(t))

	requireRefusedAsOutside(t, srv, first)
}

// TestDashboard_InRepoSymlinkChainAnswersAsTheCommand: a nine-link chain
// that ends inside the repository, within the OS's limit, answers byte-equal
// to tp export --file through it.
func TestDashboard_InRepoSymlinkChainAnswersAsTheCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	first := linkChain(t, dir, 9, "spec.tasks.json")
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	requireSameAsCommand(t, srv, dir, "/api/export?file="+first, "export", "--file", first)
}


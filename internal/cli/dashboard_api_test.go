package cli_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireSameAsCommand requires a GET of path to answer 200 with exactly the
// JSON tp prints for args in dir.
func requireSameAsCommand(t *testing.T, srv *dashboardServer, dir, path string, args ...string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "%v: %s", args, stderr)
	status, body := getPage(t, strings.TrimSuffix(srv.url, "/")+path)
	require.Equal(t, http.StatusOK, status, "GET %s: %s", path, body)
	assert.Equal(t, stdout, body, "GET %s differs from tp %v", path, args)
}

// TestDashboard_GetReturnsTheCommandsJSON is §2's GET: one task file's
// tp export, a slice's --history (whole and narrowed) and an epic's
// --of-epic history arrive byte-equal to what the commands print.
func TestDashboard_GetReturnsTheCommandsJSON(t *testing.T) {
	t.Parallel()
	exportDir := exportRepo(t)
	srv := startDashboard(t, exportDir, freePort(t))
	requireSameAsCommand(t, srv, exportDir, "/api/export?file=spec.tasks.json", "export", "--file", "spec.tasks.json")

	dir, _ := twoRoundSlice(t)
	srv = startDashboard(t, dir, freePort(t))
	requireSameAsCommand(t, srv, dir, "/api/history?milestone=A", "epic", "review", "--milestone", "A", "--history")
	requireSameAsCommand(t, srv, dir, "/api/history?milestone=A&from=1&to=2", "epic", "review", "--milestone", "A", "--history", "--from", "1", "--to", "2")
	requireSameAsCommand(t, srv, dir, "/api/epic-history?epic=b", "epic", "review", "--history", "--of-epic", "b")
	requireSameAsCommand(t, srv, dir, "/api/epic-history?epic=a", "epic", "review", "--history", "--of-epic", "a")
}

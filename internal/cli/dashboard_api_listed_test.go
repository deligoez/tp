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

// listedPaths is every row's path in tp export --list, run in dir with
// extra appended to the environment.
func listedPaths(t *testing.T, dir string, extra []string) []string {
	t.Helper()
	stdout, stderr, code := runTPEnv(t, dir, extra, "export", "--list")
	require.Equal(t, 0, code, stderr)
	var list struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &list), stdout)
	paths := make([]string, 0, len(list.Files))
	for _, f := range list.Files {
		paths = append(paths, f.Path)
	}
	return paths
}

// requireNotListed requires GET /api/export?file=<file> to answer 404 with
// code 4 and a hint naming tp export --list, and no byte of secret.
func requireNotListed(t *testing.T, srv *dashboardServer, file, secret string) {
	t.Helper()
	path := "/api/export?file=" + url.QueryEscape(file)
	_, body := getPage(t, strings.TrimSuffix(srv.url, "/")+path)
	assert.NotContains(t, body, secret, file)
	e := getDashboardError(t, srv, path, http.StatusNotFound)
	assert.Equal(t, 4, e.Code, file)
	assert.Contains(t, e.Hint, "tp export --list", file)
}

// TestDashboard_ExportAnswersOnlyAPathTheListLists: /api/export takes a
// file only as tp export --list prints its path. Round 6's repro, a path
// the OS resolves inside the repository through a symlink while the spec
// beside it is read by the lexical path outside, is refused as not listed
// with no byte of the outside spec, and so is every other spelling: a
// relative path out, the absolute path of a listed file, a ./ prefix, a
// missing file and no file at all.
func TestDashboard_ExportAnswersOnlyAPathTheListLists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "d1", "d2"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join("d1", "d2"), filepath.Join(dir, "a")))
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	inner := strings.Replace(string(data), `"spec":"spec.md"`, `"spec":"secret.md"`, 1)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "X"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "X", "x.tasks.json"), []byte(inner), 0o600))
	const secret = "outside-the-repository-secret"
	outside := filepath.Join(filepath.Dir(dir), "X")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.md"), []byte("# "+secret+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "x.tasks.json"), []byte(inner), 0o600))
	initGitRepo(t, dir)
	require.Contains(t, listedPaths(t, dir, nil), "X/x.tasks.json")
	srv := startDashboard(t, dir, freePort(t))

	for _, file := range []string{
		"a/../../X/x.tasks.json",
		"../" + filepath.Base(dir) + "/X/x.tasks.json",
		"../X/x.tasks.json",
		filepath.Join(dir, "X", "x.tasks.json"),
		"./X/x.tasks.json",
		"./spec.tasks.json",
		"missing.tasks.json",
		"",
	} {
		requireNotListed(t, srv, file, secret)
	}
}


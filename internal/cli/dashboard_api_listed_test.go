package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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

// TestDashboard_ExportAnswersEveryListedPathAsTheCommand: every path tp
// export --list prints answers byte-equal to tp export --file <path>, from
// the repository's directory and from a working directory reached through
// a symlink.
func TestDashboard_ExportAnswersEveryListedPathAsTheCommand(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	writeTaskFileRaw(t, filepath.Join(dir, "sub"), `[]`)
	link := filepath.Join(t.TempDir(), "linked")
	require.NoError(t, os.Symlink(dir, link))

	for _, run := range []struct {
		cwd string
		env []string
	}{{dir, nil}, {link, []string{"PWD=" + link}}} {
		srv := startDashboardEnv(t, run.cwd, freePort(t), run.env)
		paths := listedPaths(t, run.cwd, run.env)
		require.ElementsMatch(t, []string{"spec.tasks.json", "sub/spec.tasks.json"}, paths)
		for _, p := range paths {
			stdout, stderr, code := runTPEnv(t, run.cwd, run.env, "export", "--file", p)
			require.Equal(t, 0, code, stderr)
			status, body := getPage(t, strings.TrimSuffix(srv.url, "/")+"/api/export?file="+url.QueryEscape(p))
			require.Equal(t, http.StatusOK, status, body)
			assert.Equal(t, stdout, body, "GET /api/export?file=%s from %s differs from tp export --file", p, run.cwd)
		}
	}
}

// TestDashboard_ExportServesATaskFileAddedWhileItRuns: the list is derived
// at each request, so a task file written after the server started is
// fetchable once tp export --list lists it.
func TestDashboard_ExportServesATaskFileAddedWhileItRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))
	requireNotListed(t, srv, "later/spec.tasks.json", "\x00")

	require.NoError(t, os.Mkdir(filepath.Join(dir, "later"), 0o755))
	writeTaskFileRaw(t, filepath.Join(dir, "later"), `[]`)
	require.Contains(t, listedPaths(t, dir, nil), "later/spec.tasks.json")
	requireSameAsCommand(t, srv, dir, "/api/export?file=later%2Fspec.tasks.json", "export", "--file", "later/spec.tasks.json")
}

// TestDashboard_ExportReadsAListedPathFromTheRepositoryRoot: list paths are
// relative to the repository root, so a server started in a subdirectory
// answers each listed path as tp export --file <path> run at the root, never
// the file the path would name from its own directory: the ignored
// sub/sub/spec.tasks.json, whose spec holds a secret, is not served for
// ?file=sub/spec.tasks.json, listed or deleted, and the deleted one answers
// the command's error byte for byte, its message naming sub/spec.tasks.json.
func TestDashboard_ExportReadsAListedPathFromTheRepositoryRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	sub := filepath.Join(dir, "sub")
	nested := filepath.Join(sub, "sub")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	writeTaskFileRaw(t, sub, `[]`)
	writeTaskFileRaw(t, nested, `[]`)
	const secret = "ignored-nested-task-file-secret"
	require.NoError(t, os.WriteFile(filepath.Join(nested, "spec.md"), []byte("# "+secret+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("sub/sub/\n"), 0o600))
	initGitRepo(t, dir)
	paths := listedPaths(t, dir, nil)
	require.ElementsMatch(t, []string{"spec.tasks.json", "sub/spec.tasks.json"}, paths)
	srv := startDashboard(t, sub, freePort(t))

	for _, p := range paths {
		requireSameAsCommand(t, srv, dir, "/api/export?file="+url.QueryEscape(p), "export", "--file", p)
	}

	require.NoError(t, os.Remove(filepath.Join(sub, "spec.tasks.json")))
	require.Contains(t, listedPaths(t, dir, nil), "sub/spec.tasks.json")
	_, stderr, code := runTP(t, dir, "export", "--file", "sub/spec.tasks.json")
	require.Equal(t, 3, code, stderr)
	var want dashboardError
	require.NoError(t, json.Unmarshal([]byte(stderr), &want), stderr)
	path := "/api/export?file=sub%2Fspec.tasks.json"
	_, body := getPage(t, strings.TrimSuffix(srv.url, "/")+path)
	assert.NotContains(t, body, secret)
	assert.Equal(t, want, getDashboardError(t, srv, path, http.StatusNotFound))
}

// TestDashboard_ExportFromBelowTheRootMatchesTheRootCommand: a server
// started in a subdirectory, directly or through a symlink to it, answers
// /api/export?file=<p> byte-equal to tp export --file <p> run at the
// repository root, an uncommitted change to the task file included, so the
// paths, brief commands, keep-list and closure entries read as at the root.
func TestDashboard_ExportFromBelowTheRootMatchesTheRootCommand(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	_, stderr, code := runTP(t, dir, "add", `{"id":"t2","title":"Y","description":"the second task","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"y done","source_sections":["## 2. Y"]}`)
	require.Equal(t, 0, code, stderr)
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "notes.txt"), []byte("x\n"), 0o600))
	link := filepath.Join(t.TempDir(), "linked")
	require.NoError(t, os.Symlink(sub, link))
	want, stderr, code := runTP(t, dir, "export", "--file", "spec.tasks.json")
	require.Equal(t, 0, code, stderr)

	for _, run := range []struct {
		cwd string
		env []string
	}{{sub, nil}, {link, []string{"PWD=" + link}}} {
		srv := startDashboardEnv(t, run.cwd, freePort(t), run.env)
		status, body := getPage(t, strings.TrimSuffix(srv.url, "/")+"/api/export?file=spec.tasks.json")
		require.Equal(t, http.StatusOK, status, body)
		assert.Equal(t, want, body, "GET /api/export?file=spec.tasks.json from %s differs from tp export --file run at the root", run.cwd)
	}
}

// TestDashboard_ExportReadsNoOtherTaskFile: whether a path is listed is
// decided from the paths alone, so a sibling task file that would block any
// reader (a symlink to a FIFO no one writes) leaves /api/export of a listed
// file answering at once, byte-equal to tp export --file.
func TestDashboard_ExportReadsNoOtherTaskFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	fifo := filepath.Join(t.TempDir(), "blocked.tasks.json")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	require.NoError(t, os.Symlink(fifo, filepath.Join(dir, "blocked.tasks.json")))
	initGitRepo(t, dir)
	srv := startDashboard(t, dir, freePort(t))

	stdout, stderr, code := runTP(t, dir, "export", "--file", "spec.tasks.json")
	require.Equal(t, 0, code, stderr)
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimSuffix(srv.url, "/") + "/api/export?file=spec.tasks.json") //nolint:noctx // a loopback URL the test's own child printed
	require.NoError(t, err, "GET /api/export blocked on a sibling task file")
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	assert.Equal(t, stdout, string(data))
}

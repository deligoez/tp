package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dashboardEnv is the inherited environment with TP_UNATTENDED removed, so a
// dashboard test serves even when the suite runs inside a tp run unit, and
// re-added only when unattended is true.
func dashboardEnv(unattended bool) []string {
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+3)
	for _, kv := range inherited {
		if !strings.HasPrefix(kv, "TP_UNATTENDED=") {
			env = append(env, kv)
		}
	}
	env = append(env, "NO_COLOR=1", "TP_HC=0")
	if unattended {
		env = append(env, "TP_UNATTENDED=1")
	}
	return env
}

// freePort returns a loopback port no listener holds at the time of the call.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// lockedBuffer is a bytes.Buffer safe to read while a child's stderr copier
// writes it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// dashboardServer is a running tp dashboard child and the URL it printed.
type dashboardServer struct {
	cmd    *exec.Cmd
	url    string
	stderr *lockedBuffer
}

// startDashboard starts tp dashboard on port in dir, decodes the URL it
// prints, and stops the child when the test ends.
func startDashboard(t *testing.T, dir string, port int) *dashboardServer {
	t.Helper()
	cmd := exec.Command(binaryPath, "--json", "dashboard", "--port", strconv.Itoa(port))
	cmd.Dir = dir
	cmd.Env = dashboardEnv(false)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	var printed struct {
		URL string `json:"url"`
	}
	decoded := make(chan error, 1)
	go func() { decoded <- json.NewDecoder(stdout).Decode(&printed) }()
	select {
	case err := <-decoded:
		require.NoError(t, err, "stderr: %s", stderr.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("tp dashboard printed no URL; stderr: %s", stderr.String())
	}
	return &dashboardServer{cmd: cmd, url: printed.URL, stderr: stderr}
}

// runDashboardOnce runs tp dashboard expecting it to exit rather than serve,
// failing the test if it is still running after ten seconds.
func runDashboardOnce(t *testing.T, dir string, unattended bool, args ...string) (stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, append([]string{"--json", "dashboard"}, args...)...)
	cmd.Dir = dir
	cmd.Env = dashboardEnv(unattended)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	require.NoError(t, ctx.Err(), "tp dashboard kept serving; stderr: %s", errBuf.String())
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return errBuf.String(), code
}

// getPage fetches url and returns its status and body.
func getPage(t *testing.T, url string) (status int, body string) {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec,noctx // a loopback URL the test's own child printed
	require.NoError(t, err)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, string(data)
}

// TestDashboard_ServesThePageOnTheLoopbackURL is §1's first acceptance and
// §18 row 1's bind: in a repository with a graph tp dashboard prints
// http://127.0.0.1:<port>/, a GET of it returns the page, a second instance
// on the same port exits 1 naming the port, and nothing answers on a
// non-loopback address of this host.
func TestDashboard_ServesThePageOnTheLoopbackURL(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	port := freePort(t)
	srv := startDashboard(t, dir, port)
	require.Equal(t, fmt.Sprintf("http://127.0.0.1:%d/", port), srv.url)

	status, body := getPage(t, srv.url)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "<title>tp dashboard</title>")

	stderr, code := runDashboardOnce(t, dir, false, "--port", strconv.Itoa(port))
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, strconv.Itoa(port))

	if ip := nonLoopbackIPv4(t); ip != "" {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), time.Second)
		if err == nil {
			_ = conn.Close()
		}
		require.Error(t, err, "the server answers on %s, not on 127.0.0.1 alone", ip)
	}
}

// TestDashboard_GuardsTheServerOnARepoWithOneTaskFile is §18 row 1 on its
// own fixture, a repository with one task file: the running server answers
// GET alone, refuses a foreign Host, sends no CORS header, listens on
// 127.0.0.1 only, and a second instance on its port exits 1 naming it.
func TestDashboard_GuardsTheServerOnARepoWithOneTaskFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	port := freePort(t)
	srv := startDashboard(t, dir, port)

	status, _ := getPage(t, srv.url)
	require.Equal(t, http.StatusOK, status)

	post, err := http.Post(srv.url, "text/plain", strings.NewReader("x")) //nolint:gosec,noctx // the test's own loopback child
	require.NoError(t, err)
	require.NoError(t, post.Body.Close())
	assert.Equal(t, http.StatusMethodNotAllowed, post.StatusCode)

	req, err := http.NewRequest(http.MethodGet, srv.url, http.NoBody) //nolint:noctx // the test's own loopback child
	require.NoError(t, err)
	req.Host = "rebound.example:" + strconv.Itoa(port)
	req.Header.Set("Origin", "http://rebound.example")
	foreign, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, foreign.Body.Close())
	assert.Equal(t, http.StatusForbidden, foreign.StatusCode)
	for name := range foreign.Header {
		assert.False(t, strings.HasPrefix(strings.ToLower(name), "access-control-"), "CORS header %s", name)
	}

	stderr, code := runDashboardOnce(t, dir, false, "--port", strconv.Itoa(port))
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, strconv.Itoa(port))

	if ip := nonLoopbackIPv4(t); ip != "" {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), time.Second)
		if err == nil {
			_ = conn.Close()
		}
		require.Error(t, err, "the server answers on %s, not on 127.0.0.1 alone", ip)
	}
}

// nonLoopbackIPv4 returns an IPv4 address of this host that is not loopback,
// or "" when it has none.
func nonLoopbackIPv4(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			return n.IP.String()
		}
	}
	return ""
}

// TestDashboard_UnattendedExitsTwo is §1's second acceptance, first half: a
// long-running server is not a unit's job.
func TestDashboard_UnattendedExitsTwo(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	stderr, code := runDashboardOnce(t, dir, true, "--port", strconv.Itoa(freePort(t)))
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "TP_UNATTENDED")
}

// TestDashboard_NeedsAGraphOrATaskFile is §18 row 2's start: an empty
// directory exits 3 with a hint, and a directory with one task file and no
// graph serves.
func TestDashboard_NeedsAGraphOrATaskFile(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	stderr, code := runDashboardOnce(t, empty, false, "--port", strconv.Itoa(freePort(t)))
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `"hint"`)
	assert.Contains(t, stderr, "tp init")

	withTasks := t.TempDir()
	writeTaskFileRaw(t, withTasks, `[]`)
	srv := startDashboard(t, withTasks, freePort(t))
	status, _ := getPage(t, srv.url)
	require.Equal(t, http.StatusOK, status)
}

// TestDashboard_WritesNothingToTheRepository is §1's third acceptance: after
// the server has run and answered, git status shows no change it made.
func TestDashboard_WritesNothingToTheRepository(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	require.NoError(t, os.Remove(filepath.Join(dir, ".git")))
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	require.Empty(t, gitRun(t, dir, "status", "--porcelain", "--ignored"))

	srv := startDashboard(t, dir, freePort(t))
	status, _ := getPage(t, srv.url)
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, srv.cmd.Process.Signal(os.Interrupt))
	require.NoError(t, srv.cmd.Wait(), "an interrupted server exits 0; stderr: %s", srv.stderr.String())

	assert.Empty(t, gitRun(t, dir, "status", "--porcelain", "--ignored"))
}

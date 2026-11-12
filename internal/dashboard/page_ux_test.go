package dashboard_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runPure runs prog under node after the page's pure.js, with its functions
// bound to P, and returns what prog writes. Without node it skips locally
// and fails under CI, where it must run.
func runPure(t *testing.T, prog string) string {
	t.Helper()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", "/pure.js")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	node, err := exec.LookPath("node")
	if err != nil && os.Getenv("CI") != "" {
		t.Fatal("node is not on PATH under CI (CI is set, as GitHub Actions sets CI=true): the page's pure functions must run there")
	}
	if err != nil {
		t.Skip("node is not on PATH")
	}
	src := string(body) + "\nconst P = module.exports;\nconst out = (v) => process.stdout.write(JSON.stringify(v));\n" + prog
	got, err := exec.CommandContext(t.Context(), node, "-e", src).CombinedOutput()
	require.NoError(t, err, string(got))
	return string(got)
}

// TestPage_LoadsThePureFunctionsBeforeTheScript: the page's pure functions
// live in pure.js, which the page loads before app.js so the script finds
// them, and which node loads alone in the tests.
func TestPage_LoadsThePureFunctionsBeforeTheScript(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	pure := strings.Index(page, `src="pure.js"`)
	app := strings.Index(page, `src="app.js"`)
	require.GreaterOrEqual(t, pure, 0, "the page loads pure.js")
	assert.Less(t, pure, app, "pure.js loads before app.js")
}

// TestPure_TheFirstViewFitsTheWidthAtAReadableScale is the map's first view
// (spec/backlog/dashboard-ux.md §2): the drawing's width fits the box, never
// below the readable scale, where the view starts at the top left and the
// rest is panned to, and never above the largest scale, so a small drawing is
// not blown up.
func TestPure_TheFirstViewFitsTheWidthAtAReadableScale(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.initialView(1184, 3000, 1400, 660), P.initialView(1184, 3000, 358, 500), P.initialView(200, 100, 1400, 660)]);`)
	assert.JSONEq(t, `[
		{"x": -8, "y": -8, "w": 1200, "h": 565.7142857142857},
		{"x": -8, "y": -8, "w": 358, "h": 500},
		{"x": -8, "y": -8, "w": 933.3333333333334, "h": 440}
	]`, got)
}

// TestPage_TheMapGoesFullScreen is the map's full-screen control
// (spec/backlog/dashboard-ux.md §2): a toolbar button asks the Fullscreen API
// for the page and marks the map to fill it, leaving full screen (Esc, which
// the browser handles) unmarks it, and while full screen the plain wheel
// zooms, as Ctrl or ⌘ with the wheel does outside it.
func TestPage_TheMapGoesFullScreen(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `id="f-full"`)
	for _, s := range []string{"requestFullscreen(", "exitFullscreen(", `"fullscreenchange"`, "document.fullscreenElement", `"mapfs"`} {
		assert.Contains(t, script, s)
	}
	assert.Contains(t, page, "body.mapfs")
}

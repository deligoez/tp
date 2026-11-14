package dashboard_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/dashboard"
)

// indexFor is the page the handler serves for project.
func indexFor(t *testing.T, project string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7377/", http.NoBody)
	req.Host = "127.0.0.1:7377"
	rec := httptest.NewRecorder()
	dashboard.Handler(testPort, project, nil).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Result().Body)
	require.NoError(t, err)
	return string(body)
}

// TestHandler_NamesTheProjectBetweenTpAndDashboard: the brand and the tab
// title read "tp <project> dashboard", so two open dashboards tell apart.
func TestHandler_NamesTheProjectBetweenTpAndDashboard(t *testing.T) {
	t.Parallel()
	page := indexFor(t, "acme-shop")
	assert.Contains(t, page, `<span class="tp">tp</span><span class="project">acme-shop</span><span>dashboard</span>`)
	assert.Contains(t, page, `<title>tp acme-shop dashboard</title>`)
}

// TestHandler_EscapesTheProjectName: a directory name is not markup.
func TestHandler_EscapesTheProjectName(t *testing.T) {
	t.Parallel()
	page := indexFor(t, `a<b>&"c`)
	assert.Contains(t, page, `<span class="project">a&lt;b&gt;&amp;&#34;c</span>`)
	assert.Contains(t, page, `<title>tp a&lt;b&gt;&amp;&#34;c dashboard</title>`)
	assert.NotContains(t, page, `a<b>`)
}

// TestHandler_WithoutAProjectKeepsThePlainBrand: no name, no empty span.
func TestHandler_WithoutAProjectKeepsThePlainBrand(t *testing.T) {
	t.Parallel()
	page := indexFor(t, "")
	assert.Contains(t, page, `<span class="tp">tp</span><span>dashboard</span>`)
	assert.Contains(t, page, `<title>tp dashboard</title>`)
	assert.NotContains(t, page, `class="project"`)
}

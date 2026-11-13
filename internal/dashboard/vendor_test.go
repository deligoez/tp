package dashboard

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deckSHA256 is the sha256 of deck.gl 9.3.12's dist.min.js as npm ships it
// (page/vendor/PROVENANCE.md says how it was fetched and checked).
const deckSHA256 = "9a58a99e0590185cd4e12acbfece94cd77ad433688a5b27f184731704d03f108"

// readPage reads one embedded page file.
func readPage(t *testing.T, name string) []byte {
	t.Helper()
	b, err := fs.ReadFile(page, "page/"+name)
	require.NoError(t, err, name)
	return b
}

// gunzip is b decompressed.
func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	require.NoError(t, err)
	out, err := io.ReadAll(zr)
	require.NoError(t, err)
	return out
}

// TestVendor_DeckGLIsTheUpstreamFileWithItsLicence pins the vendored
// renderer (decision 3 of spec/backlog/dashboard-graph.md): stored
// gzip-compressed, it decompresses to the exact bytes npm ships, its
// licence sits beside it, and the provenance note names the version, the
// package and the same checksum, so an upgrade is a repeatable step.
func TestVendor_DeckGLIsTheUpstreamFileWithItsLicence(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256(gunzip(t, readPage(t, "vendor/deckgl-9.3.12.min.js.gz")))
	assert.Equal(t, deckSHA256, hex.EncodeToString(sum[:]))
	assert.Contains(t, string(readPage(t, "vendor/LICENSE-deckgl")), "Vis.gl contributors")
	prov := string(readPage(t, "vendor/PROVENANCE.md"))
	for _, s := range []string{"deck.gl@9.3.12", "dist.min.js", deckSHA256, "https://registry.npmjs.org/deck.gl/-/deck.gl-9.3.12.tgz", "sha512-"} {
		assert.Contains(t, prov, s)
	}
	_, err := fs.Stat(page, "page/vendor/deckgl-9.3.12.min.js")
	assert.ErrorIs(t, err, fs.ErrNotExist, "only the compressed copy is embedded")
}

// TestHandler_ServesTheRendererGzippedOrPlain: a client that accepts gzip
// gets the stored bytes with Content-Encoding gzip, any other the plain
// file, both as JavaScript and both varying on Accept-Encoding.
func TestHandler_ServesTheRendererGzippedOrPlain(t *testing.T) {
	t.Parallel()
	get := func(accept string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7377/vendor/deckgl-9.3.12.min.js", http.NoBody)
		req.Host = "127.0.0.1:7377"
		if accept != "" {
			req.Header.Set("Accept-Encoding", accept)
		}
		rec := httptest.NewRecorder()
		Handler(7377, nil).ServeHTTP(rec, req)
		return rec.Result()
	}
	for _, accept := range []string{"gzip, deflate, br", ""} {
		resp := get(accept)
		require.Equal(t, http.StatusOK, resp.StatusCode, accept)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, resp.Header.Get("Content-Type"), "javascript", accept)
		assert.Equal(t, "Accept-Encoding", resp.Header.Get("Vary"), accept)
		if accept != "" {
			assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))
			assert.Equal(t, readPage(t, "vendor/deckgl-9.3.12.min.js.gz"), body)
			body = gunzip(t, body)
		} else {
			assert.Empty(t, resp.Header.Get("Content-Encoding"))
		}
		sum := sha256.Sum256(body)
		assert.Equal(t, deckSHA256, hex.EncodeToString(sum[:]), accept)
	}
}

// getWith answers one GET of path with the given request headers.
func getWith(path string, headers map[string]string) *http.Response {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7377"+path, http.NoBody)
	req.Host = "127.0.0.1:7377"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	Handler(7377, nil).ServeHTTP(rec, req)
	return rec.Result()
}

// TestHandler_ServesOnlyTheRendererUnderVendor: vendor/ answers the
// renderer's .js route alone; its stored .gz, its licence and its
// provenance note are embedded for the binary's readers, not served.
func TestHandler_ServesOnlyTheRendererUnderVendor(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/vendor/deckgl-9.3.12.min.js.gz", "/vendor/LICENSE-deckgl", "/vendor/PROVENANCE.md", "/vendor/", "/vendor/nope.js"} {
		assert.Equal(t, http.StatusNotFound, getWith(path, nil).StatusCode, path)
	}
	assert.Equal(t, http.StatusOK, getWith("/vendor/deckgl-9.3.12.min.js", nil).StatusCode)
}

// TestHandler_TheRendererCarriesAnETagAndCachesForever: the renderer's
// name holds its version, so it is cached as immutable; each encoding has
// its own ETag, and a request naming it is answered 304 with no body.
func TestHandler_TheRendererCarriesAnETagAndCachesForever(t *testing.T) {
	t.Parallel()
	const path = "/vendor/deckgl-9.3.12.min.js"
	gz := getWith(path, map[string]string{"Accept-Encoding": "gzip"})
	plain := getWith(path, nil)
	for _, resp := range []*http.Response{gz, plain} {
		assert.Equal(t, "public, max-age=31536000, immutable", resp.Header.Get("Cache-Control"))
		assert.NotEmpty(t, resp.Header.Get("ETag"))
	}
	assert.NotEqual(t, gz.Header.Get("ETag"), plain.Header.Get("ETag"))
	again := getWith(path, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": gz.Header.Get("ETag")})
	assert.Equal(t, http.StatusNotModified, again.StatusCode)
	body, err := io.ReadAll(again.Body)
	require.NoError(t, err)
	assert.Empty(t, body)
}

// TestPage_DeckGLIsVendoredAndLoadedLazily: no page file loads a script or
// stylesheet from the network, and the page names the vendored file only
// in graph.js, which adds it when the card first scrolls into view, never
// as a static script in index.html.
func TestPage_DeckGLIsVendoredAndLoadedLazily(t *testing.T) {
	t.Parallel()
	read := func(name string) string { return string(readPage(t, name)) }
	remote := regexp.MustCompile(`(?i)<(script|link)[^>]+(src|href)="https?://`)
	for _, name := range []string{"index.html", "app.js", "graph.js", "pure.js"} {
		assert.False(t, remote.MatchString(read(name)), "%s loads from the network", name)
	}
	index := read("index.html")
	assert.NotContains(t, index, "deckgl", "index.html must not load deck.gl up front")
	graph := read("graph.js")
	assert.Contains(t, graph, `"vendor/deckgl-9.3.12.min.js"`)
	assert.Contains(t, graph, "IntersectionObserver", "deck.gl loads when the card is on screen")
	assert.Contains(t, graph, `getContext("webgl2")`, "a browser without WebGL2 is told so")
	assert.Greater(t, strings.Index(index, `src="graph.js"`), strings.Index(index, `src="app.js"`), "graph.js reads tpApp, so it loads after app.js")
}

// TestAcceptsGzip: gzip counts when it is named with no quality or one
// above zero, in any case, among other codings; q=0 refuses it.
func TestAcceptsGzip(t *testing.T) {
	t.Parallel()
	for header, want := range map[string]bool{
		"gzip": true, "deflate, GZIP": true, "br;q=1, gzip;q=0.5": true, "gzip; q=0": false,
		"gzip;q=0.0": false, "": false, "br": false, "x-gzip": false,
		"gzip;Q=0": false, "gzip; xq=0": true, "gzip;level=9;q=0.5": true, "gzip;q=0;x=1": false,
	} {
		assert.Equal(t, want, acceptsGzip(header), header)
	}
}

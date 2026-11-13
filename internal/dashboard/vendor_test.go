package dashboard

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPage_DeckGLIsVendoredAndLoadedLazily: the graph card's renderer is
// embedded with its license, no page file loads a script or stylesheet from
// the network, and the page names the vendored file only in graph.js, which
// adds it when the card first scrolls into view, never as a static script.
func TestPage_DeckGLIsVendoredAndLoadedLazily(t *testing.T) {
	t.Parallel()
	read := func(name string) string {
		b, err := fs.ReadFile(page, "page/"+name)
		require.NoError(t, err, name)
		return string(b)
	}
	assert.Greater(t, len(read("vendor/deckgl-9.3.12.min.js")), 1_000_000, "the vendored deck.gl bundle")
	assert.Contains(t, read("vendor/LICENSE-deckgl"), "Vis.gl contributors")
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
	assert.True(t, strings.Index(index, `src="graph.js"`) > strings.Index(index, `src="app.js"`), "graph.js reads tpApp, so it loads after app.js")
}

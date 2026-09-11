package cli

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// straddlingBodies are two section bodies of two-byte runes one byte apart:
// whatever fixed-length text precedes the body, the cap falls inside a rune
// for one of them, so a byte cut is caught without restating the layout.
var straddlingBodies = map[string]string{
	"even": strings.Repeat("ı", 8000),
	"odd":  "a" + strings.Repeat("ı", 8000),
}

// assertRuneCut checks a cut text and returns the head before marker: the head
// is valid UTF-8 and the marker counts bytes.
func assertRuneCut(t *testing.T, text, marker string) string {
	t.Helper()
	at := strings.Index(text, marker)
	require.GreaterOrEqual(t, at, 0, "the text says it was cut")
	head := text[:at]
	assert.True(t, utf8.ValidString(head), "the head is valid UTF-8, ending on a rune boundary: %q", head[max(0, len(head)-6):])
	rest := text[at+1:]
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[:nl]
	}
	assert.Contains(t, rest, "bytes", "the marker counts bytes")
	assert.NotContains(t, rest, "chars", "the cap counts bytes")
	return head
}

// TestBuildDiffSpecContentCutsOnRuneBoundary: --diff-from's content was cut
// with content[:specContentCap], keeping the lead byte of a split rune.
func TestBuildDiffSpecContentCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	for name, body := range straddlingBodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := buildDiffSpecContent(&engine.DiffResult{Changed: []engine.DiffSection{
				{Heading: "Big", Status: engine.DiffModified, Content: body, Level: 2},
			}})
			head := assertRuneCut(t, got, "\n[...diff truncated")
			assert.Greater(t, len(head), specContentCap-utf8.UTFMax, "the back-off drops only the split rune")
			assert.LessOrEqual(t, len(head), specContentCap)
		})
	}
}

// TestBuildChangedSectionsBlockCutsOnRuneBoundary: the changed-sections block
// wrote sect[:remaining] at its total cap, keeping the lead byte of a split
// rune, under a marker that counted "chars".
func TestBuildChangedSectionsBlockCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	for name, body := range straddlingBodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := buildChangedSectionsBlock(&engine.DiffResult{Changed: []engine.DiffSection{
				{Heading: "Big", Status: engine.DiffModified, Content: body, Level: 2},
			}}, "round 1")
			head := assertRuneCut(t, got, "\n[...diff content truncated")
			_, body, found := strings.Cut(head, "\n### Big\n")
			require.True(t, found, "the block holds the section")
			sect := "\n### Big\n" + body
			assert.Greater(t, len(sect), diffBlockTotalByteCap-utf8.UTFMax, "the back-off drops only the split rune")
			assert.LessOrEqual(t, len(sect), diffBlockTotalByteCap)
		})
	}
}

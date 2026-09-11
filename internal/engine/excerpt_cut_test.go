package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractSpecExcerptCutsOnRuneBoundary: a source_lines excerpt was capped
// with excerpt[:maxExcerptChars], keeping the lead byte of a rune split there.
func TestExtractSpecExcerptCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// Line 1 is the whole excerpt; its bytes 1999-2000 are one "ı".
	line := strings.Repeat("a", maxExcerptChars-1) + strings.Repeat("ı", 100)
	specPath := filepath.Join(t.TempDir(), "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(line+"\n"), 0o600))

	got := ExtractSpecExcerpt(specPath, "1-1")
	head, marker, found := strings.Cut(got, "\n[...")
	require.True(t, found, "the excerpt says it was cut")
	assert.True(t, utf8.ValidString(head), "the head is valid UTF-8: %q", head[max(0, len(head)-6):])
	assert.Equal(t, line[:maxExcerptChars-1], head, "the cut backs off to the rune boundary before the cap")
	assert.Equal(t, "truncated, see spec lines 1-1]", marker)
}

// TestExtractSectionsExcerptCutsOnRuneBoundary: a source_sections excerpt had
// the same byte cut as the source_lines one.
func TestExtractSectionsExcerptCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// The excerpt is the heading line plus the body; with "## Big\n" (7
	// bytes) ahead of it, the body's bytes 1992-1993 sit at 1999-2000.
	heading := "## Big\n"
	body := strings.Repeat("a", maxExcerptChars-1-len(heading)) + strings.Repeat("ı", 100)
	specPath := filepath.Join(t.TempDir(), "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"+heading+body+"\n"), 0o600))

	got := ExtractSpecExcerptForTask(specPath, "", []string{"Big"})
	head, marker, found := strings.Cut(got, "\n[...")
	require.True(t, found, "the excerpt says it was cut")
	assert.True(t, utf8.ValidString(head), "the head is valid UTF-8: %q", head[max(0, len(head)-6):])
	assert.Equal(t, (heading + body)[:maxExcerptChars-1], head, "the cut backs off to the rune boundary before the cap")
	assert.Equal(t, "truncated, see spec sections Big]", marker)
}

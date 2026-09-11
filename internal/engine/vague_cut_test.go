package engine

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// straddlingLine is a line whose bytes 79-80 are one "ı": a context cut at
// byte 80 keeps its lead byte alone.
var straddlingLine = strings.Repeat("a", 79) + strings.Repeat("ı", 20)

// TestDuplicateLineContextCutsOnRuneBoundary: the duplicate-line finding's
// context was ctx[:80], keeping the lead byte of a rune split there.
func TestDuplicateLineContextCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	findings := CheckDuplicateConsecutiveLines([]string{straddlingLine, straddlingLine})
	require.Len(t, findings, 1)
	ctx := findings[0].Context
	assert.True(t, utf8.ValidString(ctx), "the context is valid UTF-8: %q", ctx[max(0, len(ctx)-6):])
	assert.Equal(t, straddlingLine[:79], ctx, "the cut backs off to the rune boundary before byte 80")
}

// TestDuplicateParagraphContextCutsOnRuneBoundary: the duplicate-paragraph
// finding's context had the same byte cut.
func TestDuplicateParagraphContextCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	findings := CheckDuplicateParagraphs([]string{straddlingLine, "", straddlingLine})
	require.Len(t, findings, 1)
	ctx := findings[0].Context
	assert.True(t, utf8.ValidString(ctx), "the context is valid UTF-8: %q", ctx[max(0, len(ctx)-6):])
	assert.Equal(t, straddlingLine[:79], ctx, "the cut backs off to the rune boundary before byte 80")
}

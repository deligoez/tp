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

// affectedHead splits a ReadFilesCapped value at its cut marker and checks
// the marker counts bytes.
func affectedHead(t *testing.T, got string) string {
	t.Helper()
	at := strings.Index(got, "\n[...")
	require.GreaterOrEqual(t, at, 0, "the content says it was cut")
	marker := got[at:]
	assert.Contains(t, marker, "bytes", "the marker counts bytes")
	assert.NotContains(t, marker, "chars", "the cap counts bytes")
	return got[:at]
}

// TestReadFilesCappedCutsOnRuneBoundary: both caps sliced bytes — the
// per-file cap at s[:maxPerFile] and the total cap at s[:remaining] — so a cap
// inside a multi-byte rune kept its lead byte alone.
func TestReadFilesCappedCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("per-file cap", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(dir, "per_file.go")
		// Bytes 3-4 are one "ı": a cap of 4 splits it.
		require.NoError(t, os.WriteFile(p, []byte("abc"+strings.Repeat("ı", 50)), 0o600))

		head := affectedHead(t, ReadFilesCapped([]string{p}, 4, 1000, "affected file")[p])
		assert.True(t, utf8.ValidString(head), "the head is valid UTF-8: %q", head)
		assert.Equal(t, "abc", head, "the cut backs off to the rune boundary before the cap")
	})

	t.Run("total cap", func(t *testing.T) {
		t.Parallel()
		first := filepath.Join(dir, "first.go")
		second := filepath.Join(dir, "second.go")
		require.NoError(t, os.WriteFile(first, []byte(strings.Repeat("x", 150)), 0o600))
		// 150 bytes remain for the second file; its bytes 149-150 are one "ı".
		require.NoError(t, os.WriteFile(second, []byte(strings.Repeat("y", 149)+strings.Repeat("ı", 100)), 0o600))

		got := ReadFilesCapped([]string{first, second}, 1000, 300, "affected file")
		head := affectedHead(t, got[second])
		assert.True(t, utf8.ValidString(head), "the head is valid UTF-8: %q", head[max(0, len(head)-6):])
		assert.Equal(t, strings.Repeat("y", 149), head, "the cut backs off to the rune boundary before the cap")
	})
}

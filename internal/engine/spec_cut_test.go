package engine

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuneBoundaryAtOrBefore: the cut backs off over a rune the cap splits and
// nowhere else — not off a rune that ends exactly at the cap, and not over
// bytes that were never UTF-8.
func TestRuneBoundaryAtOrBefore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		s    string
		n    int
		want int
	}{
		{"ascii", "abcdef", 3, 3},
		{"two-byte rune split after its lead byte", "abı", 3, 2},
		{"two-byte rune ending at n", "abıc", 4, 4},
		{"three-byte rune split after one byte", "a€b", 2, 1},
		{"three-byte rune split after two bytes", "a€b", 3, 1},
		{"four-byte rune split after three bytes", "a😀b", 4, 1},
		{"four-byte rune ending at n", "a😀b", 5, 5},
		{"stray continuation bytes stay", "a\x80\x80\x80\x80b", 5, 5},
		{"invalid lead byte stays", "a\xffb", 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runeBoundaryAtOrBefore(tc.s, tc.n)
			assert.Equal(t, tc.want, got)
			if utf8.ValidString(tc.s) {
				assert.True(t, utf8.ValidString(tc.s[:got]), "a valid spec keeps a valid head")
			}
		})
	}
}

// TestCapSpecContent: under the cap the content is whole and nothing is
// reported; over it the head ends on a rune boundary and the marker and the
// SpecCut carry the same counts.
func TestCapSpecContent(t *testing.T) {
	t.Parallel()
	whole := strings.Repeat("ı", SpecContentCap/2)
	got, cut := CapSpecContent(whole, "/specs/s.md")
	assert.Equal(t, whole, got, "content at the cap is not cut")
	assert.Nil(t, cut)

	over := "a" + strings.Repeat("ı", SpecContentCap/2)
	got, cut = CapSpecContent(over, "/specs/s.md")
	require.NotNil(t, cut)
	assert.Equal(t, SpecCut{Path: "/specs/s.md", KeptBytes: SpecContentCap - 1, TotalBytes: len(over)}, *cut)
	assert.Equal(t, over[:SpecContentCap-1]+"\n[...spec truncated at 9999 of 10001 bytes; read the rest at /specs/s.md]", got)
	assert.True(t, utf8.ValidString(got))
}

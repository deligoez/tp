package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadFilesContentCutsOnRuneBoundary: the documentation and testing
// perspectives read their files with s[:maxPerFile] and s[:remaining], so a
// cap inside a multi-byte rune kept its lead byte alone, under markers that
// named no unit.
func TestReadFilesContentCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("per-file cap", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(dir, "per_file.md")
		// The per-file cap is 5000 bytes; bytes 4999-5000 are one "ı".
		body := strings.Repeat("a", 4999) + strings.Repeat("ı", 100)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))

		got := readFilesContent([]string{p}, 30000)[p]
		head, marker, found := strings.Cut(got, "\n[...")
		require.True(t, found, "the content says it was cut")
		assert.True(t, utf8.ValidString(head), "the head is valid UTF-8: %q", head[max(0, len(head)-6):])
		assert.Equal(t, body[:4999], head, "the cut backs off to the rune boundary before the cap")
		assert.Equal(t, "truncated at 4999 of 5199 bytes]", marker)
	})

	t.Run("total cap", func(t *testing.T) {
		t.Parallel()
		first := filepath.Join(dir, "first.md")
		second := filepath.Join(dir, "second.md")
		require.NoError(t, os.WriteFile(first, []byte(strings.Repeat("x", 150)), 0o600))
		// 150 bytes remain for the second file; its bytes 149-150 are one "ı".
		body := strings.Repeat("y", 149) + strings.Repeat("ı", 100)
		require.NoError(t, os.WriteFile(second, []byte(body), 0o600))

		got := readFilesContent([]string{first, second}, 300)[second]
		head, marker, found := strings.Cut(got, "\n[...")
		require.True(t, found, "the content says it was cut")
		assert.True(t, utf8.ValidString(head), "the head is valid UTF-8: %q", head[max(0, len(head)-6):])
		assert.Equal(t, body[:149], head, "the cut backs off to the rune boundary before the cap")
		assert.Equal(t, "truncated by total cap at 149 of 349 bytes]", marker)
	})
}

// TestCompactAuditChecklistCutsOnRuneBoundary: --compact shortened checklist
// text with Text[:77], keeping the lead byte of a rune split there.
func TestCompactAuditChecklistCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// Bytes 76-77 are one "ı".
	text := strings.Repeat("a", 76) + strings.Repeat("ı", 20)
	result := auditResult{Checklist: []checklistEntry{{Text: text}}}

	compactAuditChecklist(&result)

	got := result.Checklist[0].Text
	assert.True(t, utf8.ValidString(got), "the shortened text is valid UTF-8: %q", got)
	assert.Equal(t, text[:76]+"...", got, "the cut backs off to the rune boundary before byte 77")
}

// TestFirstLineCappedCutsOnRuneBoundary: an advisory's detail was capped with
// s[:noticeDetailCap], keeping the lead byte of a rune split there.
func TestFirstLineCappedCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// Bytes 199-200 are one "ı".
	detail := strings.Repeat("a", noticeDetailCap-1) + strings.Repeat("ı", 50)

	got := firstLineCapped(detail)

	assert.True(t, utf8.ValidString(got), "the capped detail is valid UTF-8: %q", got[max(0, len(got)-6):])
	assert.Equal(t, detail[:noticeDetailCap-1]+"...", got, "the cut backs off to the rune boundary before the cap")
}

// TestFindingEvidenceCutsOnRuneBoundary: a finding's checklist item quoted
// its text into expected_evidence as text[:120], keeping the lead byte of a
// rune split there.
func TestFindingEvidenceCutsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// Bytes 119-120 are one "ı".
	text := strings.Repeat("a", 119) + strings.Repeat("ı", 20)

	item := specItemOf(&checklistEntry{ID: "finding-1", Type: "finding", Text: text}, nil)

	got := item.ExpectedEvidence
	assert.True(t, utf8.ValidString(got), "the evidence is valid UTF-8: %q", got[max(0, len(got)-6):])
	assert.Equal(t, "verify the fix for: "+text[:119], got, "the cut backs off to the rune boundary before byte 120")
}

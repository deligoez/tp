package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const namedFMSpec = `---
tp:
  domain: prose
  lens:
    all:
      - "Lens question one?"
# yaml comment, not a heading
table_note: "| not | a table |"
list_note: "1. not a numbered item"
---
# Real Heading
content line

## Sub Heading
1. real item one
2. real item two
`

func TestFrontmatter_ParseAndExclusion(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(namedFMSpec), 0o600))

	// tp: mapping parsed
	fm := ParseFrontmatter(specPath)
	require.True(t, fm.Present)
	assert.Equal(t, "prose", fm.Domain)
	assert.Equal(t, []string{"Lens question one?"}, fm.Lens["all"])
	assert.Equal(t, LineRange{Start: 1, End: 10}, fm.Lines)

	// Content lines exclude the block, absolute numbers preserved
	contentLines, _, err := countContentLines(specPath)
	require.NoError(t, err)
	for _, ln := range contentLines {
		assert.Greater(t, ln, 10, "no content line inside frontmatter")
	}

	// Headings exclude the block
	headings, err := ParseHeadings(specPath)
	require.NoError(t, err)
	require.Len(t, headings, 2)
	assert.Equal(t, 11, headings[0].Line, "absolute line numbers preserved")

	// Structured elements exclude the block (table row and list item inside
	// frontmatter are invisible)
	data, err := os.ReadFile(specPath)
	require.NoError(t, err)
	lines := BlankFrontmatterLines(strings.Split(string(data), "\n"))
	rows := ExtractTableRows(lines)
	assert.Empty(t, rows, "frontmatter table row not extracted")
	items := ExtractNumberedItems(lines)
	require.Len(t, items, 2, "only the real numbered items extracted")

	// DiffSections: frontmatter belongs to no section
	dr := DiffSections(lines, lines)
	for _, s := range dr.Unchanged {
		assert.NotContains(t, s.Heading, "tp:")
		assert.NotContains(t, s.Content, "domain: prose")
	}

	// spec_excerpt excludes the block
	excerpt := ExtractSpecExcerpt(specPath, "1-13")
	assert.NotContains(t, excerpt, "domain: prose")
	assert.Contains(t, excerpt, "# Real Heading")

	// No lint findings from the parse itself
	assert.Empty(t, fm.Errors)
	assert.Empty(t, fm.Warnings)
}

func TestFrontmatter_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("---\ntp: [broken\n---\n# Heading\ncontent\n"), 0o600))

	fm := ParseFrontmatter(specPath)
	assert.True(t, fm.Present, "closed block stays excluded")
	assert.Equal(t, DomainSoftware, fm.Domain, "defaults apply")
	assert.Empty(t, fm.Lens, "no lens applies")
	require.Len(t, fm.Errors, 1, "lint reports the error")
	assert.Contains(t, fm.Errors[0].Message, "YAML parse failed")

	// Block still excluded from parsers
	headings, err := ParseHeadings(specPath)
	require.NoError(t, err)
	require.Len(t, headings, 1)
	assert.Equal(t, 4, headings[0].Line)
}

func TestFrontmatter_Unterminated(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("---\ntp:\n  domain: prose\n# Heading After Unterminated\ncontent\n"), 0o600))

	fm := ParseFrontmatter(specPath)
	assert.False(t, fm.Present, "treated as content")
	require.Len(t, fm.Errors, 1, "lint error reported")
	assert.Contains(t, fm.Errors[0].Message, "never closed")

	// All lines are content: the heading inside the would-be block parses
	headings, err := ParseHeadings(specPath)
	require.NoError(t, err)
	require.Len(t, headings, 1)
	assert.Equal(t, "Heading After Unterminated", headings[0].Text)

	contentLines, _, err := countContentLines(specPath)
	require.NoError(t, err)
	assert.Contains(t, contentLines, 1, "opening --- line counts as content")
}

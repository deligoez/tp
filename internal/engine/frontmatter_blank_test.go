package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fmSpec = `---
tp:
  domain: prose
# not a heading: yaml comment
---
# Real Heading
content line
`

func TestBlankFrontmatter_ParserExclusion(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(fmSpec), 0o600))

	t.Run("ParseHeadings never yields a heading from frontmatter", func(t *testing.T) {
		headings, err := ParseHeadings(specPath)
		require.NoError(t, err)
		require.Len(t, headings, 1)
		assert.Equal(t, "Real Heading", headings[0].Text)
		assert.Equal(t, 6, headings[0].Line, "absolute line numbers preserved")
	})

	t.Run("countContentLines skips frontmatter lines", func(t *testing.T) {
		contentLines, totalLines, err := countContentLines(specPath)
		require.NoError(t, err)
		assert.Equal(t, []int{6, 7}, contentLines)
		assert.Equal(t, 7, totalLines)
	})

	t.Run("spec excerpt excludes the block", func(t *testing.T) {
		excerpt := ExtractSpecExcerpt(specPath, "1-7")
		assert.NotContains(t, excerpt, "domain: prose")
		assert.Contains(t, excerpt, "# Real Heading")
	})
}

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

const spanSpec = `# Title
intro line
## 1. First
first content
### 1.1 Sub
sub content
## 2. Second
second content
tail line
`

func TestSectionSpan_Derivation(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(spanSpec), 0o600))

	headings, err := ParseHeadings(specPath)
	require.NoError(t, err)
	_, totalLines, err := countContentLines(specPath)
	require.NoError(t, err)

	spans := sectionSpans(headings, totalLines)

	// Span ends before the next same-or-higher-level heading
	assert.Equal(t, LineRange{Start: 3, End: 6}, spans["## 1. First"], "includes its 1.1 subsection, ends before ## 2")
	assert.Equal(t, LineRange{Start: 5, End: 6}, spans["### 1.1 Sub"], "subsection span ends before the next level-2 heading")

	// Last section spans to EOF
	assert.Equal(t, LineRange{Start: 7, End: 9}, spans["## 2. Second"])

	// Top heading spans the whole file (subsections included)
	assert.Equal(t, LineRange{Start: 1, End: 9}, spans["# Title"])
}

func TestLineCoverage_SectionDerived(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(spanSpec), 0o600))

	t.Run("task with only source_sections covers its span", func(t *testing.T) {
		tf := &model.TaskFile{
			Tasks: []model.Task{
				{ID: "t1", SourceSections: []string{"# Title"}},
			},
		}
		findings := ValidateLineCoverage(tf, specPath)
		for _, f := range findings {
			assert.NotContains(t, f.Message, "uncovered", "whole-file section span covers everything")
			assert.NotContains(t, f.Message, "cannot be computed")
		}
	})

	t.Run("union with explicit source_lines", func(t *testing.T) {
		// Section ## 2 covers lines 7-9; explicit source_lines covers 1-2.
		// Lines 3-6 (## 1 block) stay uncovered and are reported.
		tf := &model.TaskFile{
			Tasks: []model.Task{
				{ID: "t1", SourceSections: []string{"## 2. Second"}},
				{ID: "t2", SourceLines: "1-2"},
			},
		}
		findings := ValidateLineCoverage(tf, specPath)
		var sawGap bool
		for _, f := range findings {
			assert.NotContains(t, f.Message, "cannot be computed", "union is non-empty")
			if f.Line == 3 {
				sawGap = true
				assert.Contains(t, f.Message, "uncovered lines 3-6")
			}
		}
		assert.True(t, sawGap, "gap between the two anchors is reported")
	})
}

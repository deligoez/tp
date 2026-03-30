package engine

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func headingsFromMarkdown(t *testing.T, md string) []*Heading {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(md))
	headings, err := ParseHeadingsFromScanner(scanner)
	require.NoError(t, err)
	return headings
}

func TestCheckHeadingHierarchy(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		wantErr  bool
	}{
		{
			name:     "h1 h2 h3 no skip",
			markdown: "# H1\n## H2\n### H3\n",
			wantErr:  false,
		},
		{
			name:     "h1 to h3 skips h2",
			markdown: "# H1\n### H3\n",
			wantErr:  true,
		},
		{
			name:     "h2 to h4 skips h3",
			markdown: "## H2\n#### H4\n",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headings := headingsFromMarkdown(t, tt.markdown)
			findings := CheckHeadingHierarchy(headings)
			if tt.wantErr {
				assert.NotEmpty(t, findings)
				assert.Equal(t, "error", findings[0].Severity)
				assert.Equal(t, "heading-hierarchy", findings[0].Rule)
			} else {
				assert.Empty(t, findings)
			}
		})
	}
}

func TestCheckEmptySections(t *testing.T) {
	tests := []struct {
		name       string
		markdown   string
		totalLines int
		wantErr    bool
	}{
		{
			name:       "heading with content",
			markdown:   "# H1\nSome content here\n## H2\nMore content\n",
			totalLines: 4,
			wantErr:    false,
		},
		{
			name:       "heading immediately followed by another heading",
			markdown:   "# H1\n## H2\nContent\n",
			totalLines: 3,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headings := headingsFromMarkdown(t, tt.markdown)
			findings := CheckEmptySections(headings, tt.totalLines)
			if tt.wantErr {
				assert.NotEmpty(t, findings)
				assert.Equal(t, "empty-section", findings[0].Rule)
			} else {
				assert.Empty(t, findings)
			}
		})
	}
}

func TestCheckDuplicateHeadings(t *testing.T) {
	tests := []struct {
		name    string
		markdown string
		wantErr bool
	}{
		{
			name:     "same heading same parent",
			markdown: "# Root\n## Child\nContent\n## Child\n",
			wantErr:  true,
		},
		{
			name:     "same heading different parent",
			markdown: "# Parent A\n## Child\n# Parent B\n## Child\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headings := headingsFromMarkdown(t, tt.markdown)
			findings := CheckDuplicateHeadings(headings)
			if tt.wantErr {
				assert.NotEmpty(t, findings)
				assert.Equal(t, "duplicate-heading", findings[0].Rule)
			} else {
				assert.Empty(t, findings)
			}
		})
	}
}

func TestCheckVagueLanguage(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		wantLen int
	}{
		{
			name:    "appropriate is vague",
			lines:   []string{"use appropriate handling"},
			wantLen: 1,
		},
		{
			name:    "no vague words",
			lines:   []string{"no vague words here"},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := CheckVagueLanguage(tt.lines)
			assert.Len(t, findings, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, "warning", findings[0].Severity)
				assert.Equal(t, "vague-language", findings[0].Rule)
			}
		})
	}
}

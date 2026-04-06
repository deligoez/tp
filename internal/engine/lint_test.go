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

func TestCheckDuplicateConsecutiveLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		count int
	}{
		{"two identical", []string{"hello", "hello"}, 1},
		{"three identical", []string{"hello", "hello", "hello"}, 2},
		{"separated by blank", []string{"hello", "", "hello"}, 0},
		{"empty consecutive", []string{"", "", ""}, 0},
		{"code block excluded", []string{"```go", "dup", "dup", "```"}, 0},
		{"code block with lang", []string{"```json", "dup", "dup", "```"}, 0},
		{"outside code block", []string{"before", "before", "```", "inside", "inside", "```"}, 1},
		{"whitespace only lines", []string{"  ", "  "}, 0},
		{"no duplicates", []string{"a", "b", "c"}, 0},
		{"long context truncated", []string{strings.Repeat("x", 100), strings.Repeat("x", 100)}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := CheckDuplicateConsecutiveLines(tt.lines)
			assert.Equal(t, tt.count, len(findings))
			for _, f := range findings {
				assert.Equal(t, "duplicate-line", f.Rule)
				assert.Equal(t, "warning", f.Severity)
				assert.LessOrEqual(t, len(f.Context), 80)
			}
		})
	}
}

func TestCheckNumberingGaps(t *testing.T) {
	tests := []struct {
		name     string
		headings []*Heading
		count    int
	}{
		{
			"no gaps",
			[]*Heading{{Level: 3, Text: "4.1 First", Line: 10}, {Level: 3, Text: "4.2 Second", Line: 20}, {Level: 3, Text: "4.3 Third", Line: 30}},
			0,
		},
		{
			"one gap",
			[]*Heading{{Level: 3, Text: "4.1 First", Line: 10}, {Level: 3, Text: "4.3 Third", Line: 30}},
			1,
		},
		{
			"multiple gaps",
			[]*Heading{{Level: 3, Text: "4.1 First", Line: 10}, {Level: 3, Text: "4.5 Fifth", Line: 50}},
			3,
		},
		{
			"mixed levels",
			[]*Heading{{Level: 2, Text: "2. Section", Line: 5}, {Level: 3, Text: "2.1 Sub", Line: 10}, {Level: 3, Text: "2.3 Sub", Line: 30}},
			1,
		},
		{
			"non-numeric ignored",
			[]*Heading{{Level: 2, Text: "Overview", Line: 1}, {Level: 2, Text: "1. First", Line: 10}, {Level: 2, Text: "2. Second", Line: 20}},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := CheckNumberingGaps(tt.headings)
			assert.Equal(t, tt.count, len(findings))
			for _, f := range findings {
				assert.Equal(t, "numbering-gap", f.Rule)
				assert.Equal(t, "warning", f.Severity)
			}
		})
	}
}

func TestCheckOrphanListItems(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		count int
		rule  string
	}{
		{"sequential", []string{"1. First", "2. Second", "3. Third"}, 0, ""},
		{"starts at 3", []string{"3. Third", "4. Fourth"}, 1, "orphan-list-item"},
		{"gap 1 to 3", []string{"1. First", "3. Third"}, 1, "orphan-list-item"},
		{"single item", []string{"1. Only one"}, 0, ""},
		{"code block excluded", []string{"```", "3. Not a list", "4. Also not", "```"}, 0, ""},
		{"non-list between", []string{"1. First", "Some text", "1. New list", "2. Second"}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := CheckOrphanListItems(tt.lines)
			assert.Equal(t, tt.count, len(findings))
			if tt.count > 0 {
				assert.Equal(t, tt.rule, findings[0].Rule)
				assert.Equal(t, "info", findings[0].Severity)
			}
		})
	}
}

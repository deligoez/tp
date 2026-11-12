package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specSectionsFixture carries a preamble, numbered sections at two levels, an
// unnumbered level-2 heading, an unnumbered level-3 heading and a fenced block
// holding a blank line and a heading-shaped line.
const specSectionsFixture = "# Title\n" +
	"\n" +
	"Intro para one.\n" +
	"\n" +
	"## 1. First\n" +
	"\n" +
	"Para A\n" +
	"continues.\n" +
	"\n" +
	"Para B\n" +
	"\n" +
	"```text\n" +
	"x\n" +
	"\n" +
	"## 9. Not a heading\n" +
	"```\n" +
	"\n" +
	"### Unnumbered detail\n" +
	"\n" +
	"Detail text\n" +
	"\n" +
	"## Non-goals\n" +
	"\n" +
	"N1\n" +
	"\n" +
	"## 2. Second\n" +
	"\n" +
	"S1\n" +
	"\n" +
	"### 2.1 Sub\n" +
	"\n" +
	"T1\n"

// paragraphAnchors flattens sections into "anchor|text" rows, section
// headings included as "anchor|# heading".
func paragraphAnchors(sections []SpecSection) []string {
	var out []string
	for _, s := range sections {
		out = append(out, s.Anchor+"|# "+s.Heading)
		for _, p := range s.Paragraphs {
			out = append(out, p.Anchor+"|"+p.Text)
		}
	}
	return out
}

// TestSpecSections_ParsesNumberedSectionsWithParagraphAnchors: a spec parses
// into its sections, §0 for the preamble, §N(.M) for a numbered heading of
// level 2 or deeper, §<slug> for an unnumbered level-2 heading; every
// paragraph is anchored `§N ¶M`, a fenced block is one paragraph however many
// blank lines it holds, and an unnumbered level-3 heading is a paragraph of
// the section it sits in.
func TestSpecSections_ParsesNumberedSectionsWithParagraphAnchors(t *testing.T) {
	t.Parallel()
	got := paragraphAnchors(SpecSections([]byte(specSectionsFixture)))
	assert.Equal(t, []string{
		"§0|# Title",
		"§0 ¶1|Intro para one.",
		"§1|# 1. First",
		"§1 ¶1|Para A\ncontinues.",
		"§1 ¶2|Para B",
		"§1 ¶3|```text\nx\n\n## 9. Not a heading\n```",
		"§1 ¶4|### Unnumbered detail",
		"§1 ¶5|Detail text",
		"§non-goals|# Non-goals",
		"§non-goals ¶1|N1",
		"§2|# 2. Second",
		"§2 ¶1|S1",
		"§2.1|# 2.1 Sub",
		"§2.1 ¶1|T1",
	}, got)

	sections := SpecSections([]byte(specSectionsFixture))
	require.Len(t, sections, 5)
	assert.Equal(t, 5, sections[1].Line, "a section's line is its heading's")
	assert.Equal(t, 7, sections[1].Paragraphs[0].Line, "a paragraph's line is its first line's")
}

// TestSpecSections_ALaterEditLeavesEveryEarlierAnchorUnchanged: adding a
// paragraph to a later section, a new numbered section, and an unnumbered
// heading that repeats an earlier one's slug changes no anchor before the
// edit; the repeated slug takes the next free suffix.
func TestSpecSections_ALaterEditLeavesEveryEarlierAnchorUnchanged(t *testing.T) {
	t.Parallel()
	before := paragraphAnchors(SpecSections([]byte(specSectionsFixture)))
	edited := strings.Replace(specSectionsFixture, "S1\n", "S1\n\nS2 added\n", 1) +
		"\n## 3. Third\n\nU1\n\n## Non-goals\n\nN2\n"
	after := paragraphAnchors(SpecSections([]byte(edited)))

	// every anchor up to and including §2 ¶1 is unchanged
	require.GreaterOrEqual(t, len(after), 12)
	assert.Equal(t, before[:12], after[:12])
	assert.Equal(t, []string{
		"§2 ¶2|S2 added",
		"§2.1|# 2.1 Sub",
		"§2.1 ¶1|T1",
		"§3|# 3. Third",
		"§3 ¶1|U1",
		"§non-goals-2|# Non-goals",
		"§non-goals-2 ¶1|N2",
	}, after[12:])
}

// TestSpecSections_FrontmatterIsNotAParagraph: a YAML frontmatter block is
// not part of the preamble, and line numbers still count it.
func TestSpecSections_FrontmatterIsNotAParagraph(t *testing.T) {
	t.Parallel()
	sections := SpecSections([]byte("---\ntp:\n  domain: prose\n---\n\nIntro\n\n## 1. A\n\nB\n"))
	assert.Equal(t, []string{"§0|# ", "§0 ¶1|Intro", "§1|# 1. A", "§1 ¶1|B"}, paragraphAnchors(sections))
	assert.Equal(t, 6, sections[0].Paragraphs[0].Line)
}

// TestSpecSections_ASpecWithNoPreambleHasNoSectionZero: a spec that opens on
// a numbered section, with no title and nothing above it, carries no empty
// §0; its first section is §1.
func TestSpecSections_ASpecWithNoPreambleHasNoSectionZero(t *testing.T) {
	got := SpecSections([]byte("## 1. First\n\nPara\n"))
	require.Len(t, got, 1)
	assert.Equal(t, "§1", got[0].Anchor)
	assert.Equal(t, "1. First", got[0].Heading)
}

package engine

import (
	"strconv"
	"strings"
	"unicode"
)

// SpecSection is one section of a spec as tp export prints it (spec/1.8.0.md
// §16): its anchor, its heading text, the 1-based line of its heading (0 for a
// preamble without a title), and its paragraphs.
type SpecSection struct {
	Anchor     string          `json:"anchor"`
	Heading    string          `json:"heading"`
	Line       int             `json:"line"`
	Paragraphs []SpecParagraph `json:"paragraphs"`
}

// SpecParagraph is one paragraph of a section: its anchor (`§N ¶M`), the
// 1-based line it opens on, and its text.
type SpecParagraph struct {
	Anchor string `json:"anchor"`
	Line   int    `json:"line"`
	Text   string `json:"text"`
}

// SpecSections splits a spec into anchored sections. The anchor rule:
//
//   - The text before the first section heading is §0; its first H1 is §0's
//     heading. A §0 with neither heading nor paragraph is left out.
//   - A heading of level 2 or deeper whose first token is a dotted number
//     opens §<number> (the rule floorAnchorsByLine uses).
//   - An unnumbered level-2 heading opens §<slug>, the heading lowercased with
//     every run of other characters than letters and digits made one hyphen.
//   - An anchor already taken earlier in the document takes the next free
//     suffix, -2, -3, ...; so an anchor depends only on the text above it,
//     and an edit below a section never changes the section's anchors.
//   - Any other heading is a paragraph of the section it sits in.
//
// A paragraph is a run of non-blank lines; a fenced block is one paragraph
// with the lines around it, whatever it holds. Paragraphs are numbered from 1
// within their section. Frontmatter is blanked, so line numbers still count
// it.
func SpecSections(data []byte) []SpecSection {
	b := specSectionBuilder{sections: []SpecSection{{Anchor: "§0"}}, taken: map[string]int{}}
	for i, line := range strings.Split(string(BlankFrontmatter(data)), "\n") {
		b.line(i+1, line)
	}
	b.flush()
	if s := b.sections[0]; s.Heading == "" && len(s.Paragraphs) == 0 {
		return b.sections[1:]
	}
	return b.sections
}

// specSectionBuilder accumulates sections line by line.
type specSectionBuilder struct {
	sections  []SpecSection
	taken     map[string]int
	block     []string
	blockLine int
	inFence   bool
}

func (b *specSectionBuilder) line(n int, line string) {
	switch {
	case b.inFence:
		b.add(n, line)
		b.inFence = !floorFenceRe.MatchString(line)
	case floorFenceRe.MatchString(line):
		b.add(n, line)
		b.inFence = true
	case strings.TrimSpace(line) == "":
		b.flush()
	default:
		b.text(n, line)
	}
}

// text places a non-blank line outside a fence: a section heading opens a
// section, the preamble's first H1 titles §0, another heading is a paragraph
// of its own, and anything else joins the current paragraph.
func (b *specSectionBuilder) text(n int, line string) {
	level, heading := parseHeadingLine(line)
	if level == 0 {
		b.add(n, line)
		return
	}
	b.flush()
	if anchor, ok := b.sectionAnchor(level, heading); ok {
		b.sections = append(b.sections, SpecSection{Anchor: anchor, Heading: heading, Line: n})
		return
	}
	if level == 1 && len(b.sections) == 1 && b.sections[0].Heading == "" {
		b.sections[0].Heading, b.sections[0].Line = heading, n
		return
	}
	b.add(n, line)
	b.flush()
}

// sectionAnchor is the anchor a heading opens, deduplicated against the
// anchors above it; ok is false for a heading that opens no section.
func (b *specSectionBuilder) sectionAnchor(level int, heading string) (string, bool) {
	if level < 2 {
		return "", false
	}
	base := ""
	if num := headingNumRe.FindString(heading); num != "" {
		base = "§" + num
	} else if level == 2 {
		base = "§" + specHeadingSlug(heading)
	} else {
		return "", false
	}
	b.taken[base]++
	if k := b.taken[base]; k > 1 {
		return base + "-" + strconv.Itoa(k), true
	}
	return base, true
}

func (b *specSectionBuilder) add(n int, line string) {
	if len(b.block) == 0 {
		b.blockLine = n
	}
	b.block = append(b.block, line)
}

// flush closes the current paragraph into the last section.
func (b *specSectionBuilder) flush() {
	if len(b.block) == 0 {
		return
	}
	s := &b.sections[len(b.sections)-1]
	s.Paragraphs = append(s.Paragraphs, SpecParagraph{
		Anchor: s.Anchor + " ¶" + strconv.Itoa(len(s.Paragraphs)+1),
		Line:   b.blockLine,
		Text:   strings.Join(b.block, "\n"),
	})
	b.block = nil
}

// specHeadingSlug lowercases a heading and turns every run of characters
// other than letters and digits into one hyphen, trimmed at both ends;
// "section" when nothing is left.
func specHeadingSlug(heading string) string {
	var sb strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(heading) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if hyphen && sb.Len() > 0 {
				sb.WriteByte('-')
			}
			sb.WriteRune(r)
			hyphen = false
			continue
		}
		hyphen = true
	}
	if sb.Len() == 0 {
		return "section"
	}
	return sb.String()
}

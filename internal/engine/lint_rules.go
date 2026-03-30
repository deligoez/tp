package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// CheckHeadingHierarchy detects heading level skips (e.g., h2 directly to h4).
func CheckHeadingHierarchy(headings []*Heading) []Finding {
	var findings []Finding
	for i := 1; i < len(headings); i++ {
		prev := headings[i-1]
		curr := headings[i]
		// A heading can go deeper by at most 1 level
		if curr.Level > prev.Level+1 {
			findings = append(findings, Finding{
				Line:     curr.Line,
				Severity: "error",
				Rule:     "heading-hierarchy",
				Message:  fmt.Sprintf("heading level %d skips to %d (expected ≤%d)", prev.Level, curr.Level, prev.Level+1),
			})
		}
	}
	return findings
}

// CheckEmptySections detects headings with no content before the next heading.
func CheckEmptySections(headings []*Heading, totalLines int) []Finding {
	var findings []Finding
	for i, h := range headings {
		start, end := HeadingContentRange(headings, i, totalLines)
		if end < start {
			findings = append(findings, Finding{
				Line:     h.Line,
				Severity: "error",
				Rule:     "empty-section",
				Message:  fmt.Sprintf("section %q has no content", h.Text),
			})
		}
	}
	return findings
}

// CheckDuplicateHeadings detects same heading text under the same parent.
func CheckDuplicateHeadings(headings []*Heading) []Finding {
	var findings []Finding
	type parentChild struct {
		parentLine int
		text       string
	}
	seen := make(map[parentChild]int) // maps to first occurrence line

	for _, h := range headings {
		parentLine := 0
		if h.Parent != nil {
			parentLine = h.Parent.Line
		}
		key := parentChild{parentLine, h.Text}
		if firstLine, exists := seen[key]; exists {
			findings = append(findings, Finding{
				Line:     h.Line,
				Severity: "error",
				Rule:     "duplicate-heading",
				Message:  fmt.Sprintf("duplicate heading %q under same parent (first at line %d)", h.Text, firstLine),
			})
		} else {
			seen[key] = h.Line
		}
	}
	return findings
}

var anchorLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`)

// CheckOrphanReferences finds [text](#anchor) links where the anchor heading doesn't exist.
func CheckOrphanReferences(lines []string, headings []*Heading) []Finding {
	anchors := make(map[string]bool)
	for _, h := range headings {
		anchors[headingToAnchor(h.Text)] = true
	}

	var findings []Finding
	for i, line := range lines {
		matches := anchorLinkRegex.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			anchor := m[2]
			if !anchors[anchor] {
				findings = append(findings, Finding{
					Line:     i + 1,
					Severity: "error",
					Rule:     "orphan-reference",
					Message:  fmt.Sprintf("internal link #%s does not match any heading", anchor),
					Context:  strings.TrimSpace(line),
				})
			}
		}
	}
	return findings
}

func headingToAnchor(text string) string {
	s := strings.ToLower(text)
	s = regexp.MustCompile(`[^\w\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	return s
}

// CheckSectionSize warns for sections over maxLines lines.
func CheckSectionSize(headings []*Heading, totalLines, maxSectionLines int) []Finding {
	var findings []Finding
	for i, h := range headings {
		start, end := HeadingContentRange(headings, i, totalLines)
		size := end - start + 1
		if size > maxSectionLines {
			findings = append(findings, Finding{
				Line:     h.Line,
				Severity: "warning",
				Rule:     "section-size",
				Message:  fmt.Sprintf("section %q is %d lines — consider splitting", h.Text, size),
			})
		}
	}
	return findings
}

// CheckSpecSize warns if total lines exceed maxLines.
func CheckSpecSize(totalLines, maxLines int) []Finding {
	if totalLines > maxLines {
		return []Finding{{
			Line:     1,
			Severity: "info",
			Rule:     "long-spec",
			Message:  fmt.Sprintf("spec is %d lines — consider splitting into modular sub-specs", totalLines),
		}}
	}
	return nil
}

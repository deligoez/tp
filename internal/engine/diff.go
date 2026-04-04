package engine

import (
	"strings"
)

// DiffSectionStatus indicates whether a section was added, modified, removed, or unchanged.
type DiffSectionStatus string

const (
	DiffModified  DiffSectionStatus = "MODIFIED"
	DiffAdded     DiffSectionStatus = "ADDED"
	DiffRemoved   DiffSectionStatus = "REMOVED"
	DiffUnchanged DiffSectionStatus = "UNCHANGED"
)

// DiffSection represents a section in the diff result.
type DiffSection struct {
	Heading string            `json:"heading"`
	Status  DiffSectionStatus `json:"status"`
	Content string            `json:"content,omitempty"` // full text for changed/added, empty for unchanged/removed
	Level   int               `json:"level"`
}

// DiffResult holds the result of comparing two specs.
type DiffResult struct {
	Changed   []DiffSection `json:"changed"`
	Removed   []DiffSection `json:"removed"`
	Unchanged []DiffSection `json:"unchanged"`
}

// section is an internal representation of a heading and its content.
type section struct {
	heading string
	level   int
	content []string // lines between this heading and the next
}

// DiffSections compares two specs (baseline and current) at the section level.
// Returns changed (MODIFIED/ADDED), removed, and unchanged sections.
func DiffSections(baseLines, currentLines []string) DiffResult {
	baseSections := parseSections(baseLines)
	currSections := parseSections(currentLines)

	baseMap := make(map[string]*section, len(baseSections))
	for i := range baseSections {
		baseMap[baseSections[i].heading] = &baseSections[i]
	}

	currMap := make(map[string]*section, len(currSections))
	for i := range currSections {
		currMap[currSections[i].heading] = &currSections[i]
	}

	var result DiffResult

	// Walk current sections in order
	for _, cs := range currSections {
		bs, exists := baseMap[cs.heading]
		if !exists {
			result.Changed = append(result.Changed, DiffSection{
				Heading: cs.heading,
				Status:  DiffAdded,
				Content: strings.Join(cs.content, "\n"),
				Level:   cs.level,
			})
			continue
		}

		if sectionsEqual(bs.content, cs.content) {
			result.Unchanged = append(result.Unchanged, DiffSection{
				Heading: cs.heading,
				Status:  DiffUnchanged,
				Level:   cs.level,
			})
		} else {
			result.Changed = append(result.Changed, DiffSection{
				Heading: cs.heading,
				Status:  DiffModified,
				Content: strings.Join(cs.content, "\n"),
				Level:   cs.level,
			})
		}
	}

	// Find removed sections (in baseline but not in current)
	for _, bs := range baseSections {
		if _, exists := currMap[bs.heading]; !exists {
			result.Removed = append(result.Removed, DiffSection{
				Heading: bs.heading,
				Status:  DiffRemoved,
				Level:   bs.level,
			})
		}
	}

	// Ensure nil slices are empty for JSON
	if result.Changed == nil {
		result.Changed = make([]DiffSection, 0)
	}
	if result.Removed == nil {
		result.Removed = make([]DiffSection, 0)
	}
	if result.Unchanged == nil {
		result.Unchanged = make([]DiffSection, 0)
	}

	return result
}

// parseSections splits lines into sections grouped by headings.
// Heading-like lines inside fenced code blocks are NOT treated as section boundaries.
func parseSections(lines []string) []section {
	var sections []section
	var current *section
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track fenced code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if current != nil {
				current.content = append(current.content, line)
			}
			continue
		}

		if inCodeBlock {
			if current != nil {
				current.content = append(current.content, line)
			}
			continue
		}

		// Check for heading
		level, text := parseHeadingLine(line)
		if level > 0 {
			sections = append(sections, section{
				heading: text,
				level:   level,
				content: make([]string, 0),
			})
			current = &sections[len(sections)-1]
			continue
		}

		// Regular content line
		if current != nil {
			current.content = append(current.content, line)
		}
		// Lines before the first heading are ignored (preamble)
	}

	return sections
}

// sectionsEqual compares two sets of content lines, ignoring leading/trailing
// whitespace per line and differences in blank line counts.
func sectionsEqual(a, b []string) bool {
	aNorm := normalizeContent(a)
	bNorm := normalizeContent(b)

	if len(aNorm) != len(bNorm) {
		return false
	}
	for i := range aNorm {
		if aNorm[i] != bNorm[i] {
			return false
		}
	}
	return true
}

// normalizeContent trims each line and removes consecutive blank lines,
// keeping at most one blank line between content lines.
func normalizeContent(lines []string) []string {
	result := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevBlank {
				result = append(result, "")
			}
			prevBlank = true
			continue
		}
		prevBlank = false
		result = append(result, trimmed)
	}
	// Trim trailing blanks
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return result
}

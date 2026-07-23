package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const maxExcerptChars = 2000

// LineRange represents a start-end line range (inclusive).
type LineRange struct {
	Start, End int
}

// ParseLineRanges parses source_lines like "4-10" or "4-10,15-20,25-30"
// into a slice of LineRange. Returns nil for empty input.
func ParseLineRanges(s string) ([]LineRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	var ranges []LineRange
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.SplitN(part, "-", 2)
		if len(segments) == 1 {
			// Single number: normalize "72" to "72-72"
			n, err := strconv.Atoi(strings.TrimSpace(segments[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid line number in %q: %w", part, err)
			}
			if n < 1 {
				return nil, fmt.Errorf("invalid line range: %d must be positive", n)
			}
			ranges = append(ranges, LineRange{Start: n, End: n})
			continue
		}
		start, err := strconv.Atoi(strings.TrimSpace(segments[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid line number in %q: %w", part, err)
		}
		end, err := strconv.Atoi(strings.TrimSpace(segments[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid line number in %q: %w", part, err)
		}
		if start > end {
			return nil, fmt.Errorf("invalid line range: start %d > end %d", start, end)
		}
		ranges = append(ranges, LineRange{Start: start, End: end})
	}
	return ranges, nil
}

// LineSet returns a set of all line numbers covered by the ranges.
func LineSet(ranges []LineRange) map[int]bool {
	set := make(map[int]bool)
	for _, r := range ranges {
		for i := r.Start; i <= r.End; i++ {
			set[i] = true
		}
	}
	return set
}

// ExtractSpecExcerpt reads lines from a spec file based on source_lines.
// Supports single range "15-42" and multi-range "15-42,50-60".
// Returns the excerpt capped at maxExcerptChars, with a truncation note if exceeded.
func ExtractSpecExcerpt(specPath, sourceLines string) string {
	if sourceLines == "" || specPath == "" {
		return ""
	}

	ranges, err := ParseLineRanges(sourceLines)
	if err != nil || len(ranges) == 0 {
		return ""
	}

	lineSet := LineSet(ranges)
	maxLine := 0
	for _, r := range ranges {
		if r.End > maxLine {
			maxLine = r.End
		}
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		return ""
	}
	fm := ParseFrontmatterBytes(data)

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Frontmatter lines are excluded from excerpts
		if fm.Present && lineNum >= fm.Lines.Start && lineNum <= fm.Lines.End {
			continue
		}
		if lineSet[lineNum] {
			lines = append(lines, scanner.Text())
		}
		if lineNum > maxLine {
			break
		}
	}

	excerpt := strings.Join(lines, "\n")
	if len(excerpt) > maxExcerptChars {
		excerpt = excerpt[:maxExcerptChars] + fmt.Sprintf("\n[...truncated, see spec lines %s]", sourceLines)
	}

	return excerpt
}

// ExtractSpecExcerptForTask returns the spec excerpt for a task's anchors.
// When sourceLines is present it drives the excerpt (existing behavior, line
// ranges). When sourceLines is empty and sourceSections is present, the excerpt
// is assembled from those sections: for each entry, its heading line plus the
// section body up to the next heading of the same or shallower level, in the
// order the task lists them, joined by a blank line. The result is capped at
// maxExcerptChars with a truncation marker. A section name that matches no
// heading contributes nothing (reported by tp validate's reference check).
func ExtractSpecExcerptForTask(specPath, sourceLines string, sourceSections []string) string {
	if sourceLines != "" {
		return ExtractSpecExcerpt(specPath, sourceLines)
	}
	if len(sourceSections) == 0 || specPath == "" {
		return ""
	}
	return extractSectionsExcerpt(specPath, sourceSections)
}

// extractSectionsExcerpt builds an excerpt from source_sections: each resolved
// section contributes its heading line and body (through the line before the
// next same/shallower heading), in listed order, joined by a blank line and
// capped at maxExcerptChars.
func extractSectionsExcerpt(specPath string, sourceSections []string) string {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return ""
	}
	headings, err := ParseHeadings(specPath)
	if err != nil || len(headings) == 0 {
		return ""
	}

	fm := ParseFrontmatterBytes(data)
	var fileLines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fileLines = append(fileLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return ""
	}
	totalLines := len(fileLines)
	spans := sectionSpans(headings, totalLines)

	sections := make([]string, 0, len(sourceSections))
	for _, entry := range sourceSections {
		resolved, ambiguous, _ := ResolveSection(entry, headings)
		if resolved == "" || ambiguous {
			continue
		}
		span, ok := spans[resolved]
		if !ok || span.Start < 1 {
			continue
		}
		end := span.End
		if end > totalLines {
			end = totalLines
		}
		var buf strings.Builder
		for ln := span.Start; ln <= end; ln++ {
			if fm.Present && ln >= fm.Lines.Start && ln <= fm.Lines.End {
				continue
			}
			buf.WriteString(fileLines[ln-1])
			buf.WriteByte('\n')
		}
		sections = append(sections, strings.TrimRight(buf.String(), "\n"))
	}

	excerpt := strings.Join(sections, "\n\n")
	if len(excerpt) > maxExcerptChars {
		excerpt = excerpt[:maxExcerptChars] + fmt.Sprintf("\n[...truncated, see spec sections %s]", strings.Join(sourceSections, ", "))
	}
	return excerpt
}

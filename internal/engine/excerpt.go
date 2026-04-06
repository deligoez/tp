package engine

import (
	"bufio"
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

	f, err := os.Open(specPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
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

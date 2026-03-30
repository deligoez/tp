package engine

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const maxExcerptChars = 2000

// ExtractSpecExcerpt reads lines from a spec file based on a source_lines range like "15-42".
// Returns the excerpt capped at maxExcerptChars, with a truncation note if exceeded.
func ExtractSpecExcerpt(specPath, sourceLines string) string {
	if sourceLines == "" || specPath == "" {
		return ""
	}

	start, end, err := parseLineRange(sourceLines)
	if err != nil {
		return ""
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
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
	}

	excerpt := strings.Join(lines, "\n")
	if len(excerpt) > maxExcerptChars {
		excerpt = excerpt[:maxExcerptChars] + fmt.Sprintf("\n[...truncated, see spec lines %s]", sourceLines)
	}

	return excerpt
}

func parseLineRange(s string) (start, end int, err error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid line range: %s", s)
	}
	start, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}
